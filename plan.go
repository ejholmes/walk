package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// The two arguments that .walk executables will be called with, in the "plan"
// or "exec" phases.
const (
	PhaseDeps = "deps"
	PhaseExec = "exec"
)

// Rule defines what a target depends on, and how to build it.
type Rule interface {
	// Dependencies returns the name of the targets that this target depends
	// on.
	Dependencies(context.Context) ([]string, error)

	// Exec executes the target.
	Exec(context.Context) error
}

// Target represents an individual node in the dependency graph.
type Target interface {
	Rule

	// Name returns the name of the target.
	Name() string
}

// NewTarget returns a new Target instance backed by a FileTarget.
func NewTarget(name string) (Target, error) {
	return newTarget(nil, os.Stderr)(name)
}

func newTarget(stdout, stderr io.Writer) func(string) (Target, error) {
	return func(name string) (Target, error) {
		t, err := newFileTarget(name)
		if err != nil {
			return nil, err
		}
		t.stdout = stdout
		t.stderr = stderr
		vt := &verboseFileTarget{
			FileTarget: t,
		}
		return vt, nil
	}
}

// Plan is used to build a graph of all the targets and their dependencies.
type Plan struct {
	// NewTarget is executed when a target is discovered, and added to the
	// graph.
	NewTarget func(string) (Target, error)

	graph *Graph
}

// Exec is a simple helper to build and execute a Plan.
func Exec(ctx context.Context, targets ...string) error {
	plan := newPlan()
	err := plan.Plan(ctx, targets...)
	if err != nil {
		return err
	}
	return plan.Exec(ctx)
}

func newPlan() *Plan {
	return &Plan{
		NewTarget: NewTarget,
		graph:     newGraph(),
	}
}

func (p *Plan) String() string {
	return p.graph.String()
}

// Plan builds the graph, starting with the given target.
func (p *Plan) Plan(ctx context.Context, targets ...string) error {
	for _, target := range targets {
		_, err := p.newTarget(ctx, target)
		if err != nil {
			return err
		}
	}

	// Add a root target, with all of the given targets as it's dependency.
	if err := p.addTarget(ctx, &rootTarget{deps: targets}); err != nil {
		return err
	}

	return p.graph.Validate()
}

func (p *Plan) addTarget(ctx context.Context, t Target) error {
	p.graph.Add(t)

	deps, err := t.Dependencies(ctx)
	if err != nil {
		return fmt.Errorf("error getting dependencies for %s: %v", t.Name(), err)
	}

	for _, d := range deps {
		dep, err := p.newTarget(ctx, d)
		if err != nil {
			return err
		}
		p.graph.Connect(t, dep)
	}

	return nil
}

func (p *Plan) newTarget(ctx context.Context, target string) (Target, error) {
	// Target already exists in the graph.
	if t := p.graph.Target(target); t != nil {
		return t, nil
	}

	t, err := p.NewTarget(target)
	if err != nil {
		return t, err
	}

	return t, p.addTarget(ctx, t)
}

// Exec executes the plan.
func (p *Plan) Exec(ctx context.Context) error {
	err := p.graph.Walk(func(t Target) error {
		return t.Exec(ctx)
	})
	return err
}

type fileBuildError struct {
	target *FileTarget
	err    error
}

func (e *fileBuildError) Error() string {
	prefix := fmt.Sprintf("error performing %s", e.target.Name())
	if e.target.walkfile != "" {
		prefix += fmt.Sprintf(" (using %s)", e.target.walkfile)
	}
	return fmt.Sprintf("%s: %v", prefix, e.err)
}

// FileTarget extends a Target that's represented by a file.
type FileTarget struct {
	// Relative path to the file.
	name string

	// The absolute path to the file.
	path string

	// path to the walkfile to use.
	walkfile string

	// the directory to use as the working directory when executing the
	// build file.
	dir string

	stdout, stderr io.Writer
}

// newFileTarget initializes and returns a new FileTarget instance.
func newFileTarget(name string) (*FileTarget, error) {
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}

	walkfile, err := walkFile(path)
	if err != nil {
		return nil, err
	}

	var dir string
	if walkfile != "" {
		dir = filepath.Dir(path)
	}

	return &FileTarget{
		name:     name,
		path:     path,
		walkfile: walkfile,
		dir:      dir,
	}, nil
}

func (t *FileTarget) Name() string {
	return t.name
}

// Exec executes the FileTarget.
func (t *FileTarget) Exec(ctx context.Context) error {
	if t.walkfile == "" {
		// It's possible for a target to simply be a static file, in which case
		// we don't need to perform a build. We do however want to ensure that
		// it exists in this case.
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.ruleCommand(ctx, PhaseExec)
	return cmd.Run()
}

func (t *FileTarget) Dependencies(ctx context.Context) ([]string, error) {
	// No .walk file, meaning it's a static dependency.
	if t.walkfile == "" {
		return nil, nil
	}

	b := new(bytes.Buffer)
	cmd := t.ruleCommand(ctx, PhaseDeps)
	cmd.Stdout = b

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var deps []string
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		path := scanner.Text()
		// Make all paths relative to the working directory.
		path, err := filepath.Rel(wd, filepath.Join(t.dir, scanner.Text()))
		if err != nil {
			return deps, err
		}
		deps = append(deps, path)
	}

	return deps, scanner.Err()
}

func (t *FileTarget) ruleCommand(ctx context.Context, subcommand string) *exec.Cmd {
	name := filepath.Base(t.path)
	cmd := exec.CommandContext(ctx, t.walkfile, subcommand, name)
	cmd.Stdout = t.stdout
	cmd.Stderr = t.stderr
	cmd.Dir = t.dir
	return cmd
}

type verboseFileTarget struct {
	*FileTarget
}

func (t *verboseFileTarget) Exec(ctx context.Context) error {
	err := t.FileTarget.Exec(ctx)
	if err != nil {
		return &fileBuildError{t.FileTarget, err}
	}
	if err == nil && t.walkfile != "" {
		fmt.Printf("%s\n", ansi("32", "%s", t.Name()))
	}
	return err
}

// walkFile returns the path to the .walk file that should be used to execute
// this target. If the target has no appropriate .walk file, then "" is
// returned.
func walkFile(path string) (string, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	try := []string{
		filepath.Join(".walk", name),                          // .walk/hello.o
		fmt.Sprintf("%s.walk", name),                          // hello.o.walk
		filepath.Join(".walk", fmt.Sprintf("default%s", ext)), // .walk/default.o
		fmt.Sprintf("default%s.walk", ext),                    // default.o.walk
	}

	for _, n := range try {
		path := filepath.Join(dir, n)
		_, err := os.Stat(path)
		if err == nil {
			return path, nil
		}
	}

	return "", nil
}

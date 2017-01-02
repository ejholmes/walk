package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// The two arguments that .walk executables will be called with, in the "plan"
// or "exec" phases.
const (
	ArgDeps = "deps"
	ArgExec = "exec"
)

// Target represents an individual node in the dependency graph.
type Target interface {
	// Name returns the name of the target.
	Name() string

	// Exec executes the target.
	Exec() error

	// Dependencies returns the name of the targets that this target depends
	// on.
	Dependencies() ([]string, error)
}

// NewTarget returns a new Target instance backed by a FileTarget.
func NewTarget(name string) (Target, error) {
	t, err := newFileTarget(name)
	if err != nil {
		return nil, err
	}
	return &verboseFileTarget{t}, nil
}

// Plan is used to build a graph of all the targets and their dependencies.
type Plan struct {
	// NewTarget is executed when a target is discovered, and added to the
	// graph.
	NewTarget func(string) (Target, error)

	graph *Graph
}

func newPlan() *Plan {
	return &Plan{
		NewTarget: NewTarget,
		graph:     newGraph(),
	}
}

// Plan builds the graph, starting with the given target.
func (p *Plan) Plan(target string) (Target, error) {
	// Target already exists in the graph.
	if t := p.graph.Target(target); t != nil {
		return t, nil
	}

	t, err := p.NewTarget(target)
	if err != nil {
		return t, err
	}

	p.graph.Add(t)

	deps, err := t.Dependencies()
	if err != nil {
		return t, fmt.Errorf("error getting dependencies for %s: %v", target, err)
	}

	for _, d := range deps {
		dep, err := p.Plan(d)
		if err != nil {
			return t, err
		}
		p.graph.Connect(t, dep)
	}

	return t, nil
}

// Exec executes the plan.
func (p *Plan) Exec() error {
	err := p.graph.Walk(func(t Target) error {
		return t.Exec()
	})
	return err
}

type fileBuildError struct {
	target *FileTarget
	err    error
}

func (e *fileBuildError) Error() string {
	prefix := fmt.Sprintf("error performing %s", e.target.Name())
	if e.target.buildfile != "" {
		prefix += fmt.Sprintf(" (using %s)", e.target.buildfile)
	}
	return fmt.Sprintf("%s: %v", prefix, e.err)
}

// FileTarget extends a Target that's represented by a file.
type FileTarget struct {
	// Relative path to the file.
	name string

	// The absolute path to the file.
	path string

	// path to the buildfile to use.
	buildfile string

	// the directory to use as the working directory when executing the
	// build file.
	buildir string
}

// newFileTarget initializes and returns a new FileTarget instance.
func newFileTarget(name string) (*FileTarget, error) {
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}

	buildfile, err := buildFile(path)
	if err != nil {
		return nil, err
	}

	var buildir string
	if buildfile != "" {
		buildir = filepath.Dir(path)
	}

	return &FileTarget{
		name:      name,
		path:      path,
		buildfile: buildfile,
		buildir:   buildir,
	}, nil
}

func (t *FileTarget) Name() string {
	return t.name
}

// Exec executes the FileTarget.
func (t *FileTarget) Exec() error {
	if t.buildfile == "" {
		// It's possible for a target to simply be a static file, in which case
		// we don't need to perform a build. We do however want to ensure that
		// it exists in this case.
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.buildCommand(ArgExec)
	return cmd.Run()
}

func (t *FileTarget) Dependencies() ([]string, error) {
	// No .build file, meaning it's a static dependency.
	if t.buildfile == "" {
		return nil, nil
	}

	cmd := t.buildCommand(ArgDeps)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := scanner.Text()
		// Make all paths relative to the working directory.
		path, err := filepath.Rel(wd, filepath.Join(t.buildir, scanner.Text()))
		if err != nil {
			return deps, err
		}
		deps = append(deps, path)
	}

	return deps, scanner.Err()
}

func (t *FileTarget) buildCommand(subcommand string) *exec.Cmd {
	name := filepath.Base(t.path)
	cmd := exec.Command(t.buildfile, subcommand, name)
	cmd.Stderr = os.Stderr
	cmd.Dir = t.buildir
	return cmd
}

type verboseFileTarget struct {
	*FileTarget
}

func (t *verboseFileTarget) Exec() error {
	err := t.FileTarget.Exec()
	if err != nil {
		return &fileBuildError{t.FileTarget, err}
	}
	if err == nil && t.buildfile != "" {
		fmt.Printf("%s\n", t.Name())
	}
	return err
}

// buildFile returns the path to the .build file that should be used to build
// this target. If the target has no appropriate .build file, then "" is
// returned.
func buildFile(path string) (string, error) {
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

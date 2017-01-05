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

// NewTarget returns a new Target instance backed by a target.
func NewTarget(name string) (Target, error) {
	return newTarget(nil, os.Stderr)(name)
}

func newTarget(stdout, stderr io.Writer) func(string) (Target, error) {
	return func(name string) (Target, error) {
		t, err := newtarget(name)
		if err != nil {
			return nil, err
		}
		t.stdout = stdout
		t.stderr = stderr
		vt := &verboseTarget{
			target: t,
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

// String implements the fmt.Stringer interface for Plan.
func (p *Plan) String() string {
	return p.graph.String()
}

// Returns an array of all of the targets. Should be called after Plan.
func (p *Plan) Dependencies(targets ...string) []Target {
	return p.graph.Dependencies(targets...)
}

// Plan builds the graph, starting with the given target. It recursively
// executes the "deps" phase of the targets rule, adding each dependency to the
// graph as their found.
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

// addTarget adds the given Target to the graph, as well as it's dependencies,
// then connects the target to it's dependency with and edge.
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

// newTarget instantiates a new Target instance using the Plan's NewTarget
// method, and adds it to the graph, if it hasn't already been added.
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

// Exec begins walking the graph, executing the "exec" phase of each targets
// Rule. Targets Exec functions are gauranteed to be called when all of the
// Targets dependencies have been fulfilled.
func (p *Plan) Exec(ctx context.Context) error {
	err := p.graph.Walk(func(t Target) error {
		return t.Exec(ctx)
	})
	return err
}

type fileBuildError struct {
	target *target
	err    error
}

func (e *fileBuildError) Error() string {
	prefix := fmt.Sprintf("error performing %s", e.target.Name())
	if e.target.rulefile != "" {
		prefix += fmt.Sprintf(" (using %s)", e.target.rulefile)
	}
	return fmt.Sprintf("%s: %v", prefix, e.err)
}

// target extends a Target that's represented by a file.
type target struct {
	// Relative path to the file.
	name string

	// The absolute path to the file.
	path string

	// path to the rulefile to use.
	rulefile string

	// the directory to use as the working directory when executing the
	// build file.
	dir string

	stdout, stderr io.Writer
}

// newtarget initializes and returns a new target instance.
func newtarget(name string) (*target, error) {
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}

	rulefile, err := ruleFile(path)
	if err != nil {
		return nil, err
	}

	var dir string
	if rulefile != "" {
		dir = filepath.Dir(path)
	}

	return &target{
		name:     name,
		path:     path,
		rulefile: rulefile,
		dir:      dir,
	}, nil
}

// Name implements the Target interface.
func (t *target) Name() string {
	return t.name
}

// Exec executes the rule with "exec" as the first argument.
func (t *target) Exec(ctx context.Context) error {
	if t.rulefile == "" {
		// It's possible for a target to simply be a static file, in which case
		// we don't need to perform a build. We do however want to ensure that
		// it exists in this case.
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.ruleCommand(ctx, PhaseExec)
	return cmd.Run()
}

// Dependencies executes the rule with "deps" as the first argument.
func (t *target) Dependencies(ctx context.Context) ([]string, error) {
	// No .walk file, meaning it's a static dependency.
	if t.rulefile == "" {
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

func (t *target) ruleCommand(ctx context.Context, phase string) *exec.Cmd {
	name := filepath.Base(t.path)
	cmd := exec.CommandContext(ctx, t.rulefile, phase, name)
	cmd.Stdout = t.stdout
	cmd.Stderr = t.stderr
	cmd.Dir = t.dir
	return cmd
}

type verboseTarget struct {
	*target
}

func (t *verboseTarget) Exec(ctx context.Context) error {
	err := t.target.Exec(ctx)
	if err != nil {
		return &fileBuildError{t.target, err}
	}
	if err == nil && t.rulefile != "" {
		fmt.Printf("%s\n", ansi("32", "%s", t.Name()))
	}
	return err
}

// ruleFile returns the path to the executable rule that should be used to
// execute this target. If the target has no appropriate .walk file, then "" is
// returned.
func ruleFile(path string) (string, error) {
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

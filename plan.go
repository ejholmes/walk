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

// These represent the possibilities for the $1 positional argument when
// executing rules.
const (
	PhaseDeps = "deps"
	PhaseExec = "exec"
)

// Rule defines what a target depends on, and how to execute it.
type Rule interface {
	// Dependencies returns the name of the targets that this target depends
	// on.
	Dependencies(context.Context) ([]string, error)

	// Exec executes the target.
	Exec(context.Context) error
}

// Target represents a target, which is usually built by a Rule. In general,
// targets are represented as paths to files on disk (e.g. "test/all" or
// "src/hello.o").
type Target interface {
	Rule

	// Name returns the name of the target.
	Name() string
}

// NewTarget returns a new Target instance.
func NewTarget(name string) (Target, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return newVerboseTarget(wd, nil, os.Stderr)(name)
}

// Plan is used to build a graph of all the targets and their dependencies. It
// offers two primary methods; `Plan` and `Exec`, which correspend to the `deps`
// and `exec` phases respectively.
type Plan struct {
	// NewTarget is executed when a target is discovered during Plan. This
	// method should return a new Target instance, to represent the named
	// target.
	NewTarget func(string) (Target, error)

	graph *Graph
}

// Exec is a simple helper to build and execute a target.
func Exec(ctx context.Context, semaphore Semaphore, targets ...string) error {
	plan := newPlan()
	err := plan.Plan(ctx, targets...)
	if err != nil {
		return err
	}
	return plan.Exec(ctx, semaphore)
}

// newPlan returns a new initialized Plan instance.
func newPlan() *Plan {
	return &Plan{
		NewTarget: NewTarget,
		graph:     newGraph(),
	}
}

// String implements the fmt.Stringer interface for Plan, which simply prints
// the targets and their dependencies.
func (p *Plan) String() string {
	return p.graph.String()
}

// Returns an array of all of the given targets dependencies. Should be called
// after Plan.
func (p *Plan) Dependencies(target string) ([]Target, error) {
	return p.graph.Dependencies(target)
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

	if err := p.graph.Validate(); err != nil {
		return err
	}

	p.graph.TransitiveReduction()

	return nil
}

// addTarget adds the given Target to the graph, as well as it's dependencies,
// then connects the target to it's dependency with an edge.
func (p *Plan) addTarget(ctx context.Context, t Target) error {
	p.graph.Add(t)

	deps, err := t.Dependencies(ctx)
	if err != nil {
		return fmt.Errorf("error getting dependencies for %s: %v", t.Name(), err)
	}

	for _, d := range deps {
		// TODO(ejholmes): Accept a semaphore and parallelize this. No
		// need to perform this serially.
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
func (p *Plan) Exec(ctx context.Context, semaphore Semaphore) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return p.graph.Walk(func(t Target) error {
		semaphore.P()
		defer semaphore.V()
		err := t.Exec(ctx)
		if err != nil {
			// Cancel all other currently executing targets.
			cancel()
		}
		return err
	})
}

// targetError is an error implementation that provides additional information
// about the rule that was used to build the target (if any).
type targetError struct {
	target *target
	err    error
}

func (e *targetError) Error() string {
	prefix := fmt.Sprintf("error performing %s", e.target.Name())
	if e.target.rulefile != "" {
		prefix += fmt.Sprintf(" (using %s)", e.target.rulefile)
	}
	return fmt.Sprintf("%s: %v", prefix, e.err)
}

// target is a Target implementation, that represents a file on disk, which may
// be built by a rule.
type target struct {
	// Relative path to the file.
	name string

	// The absolute path to the file.
	path string

	// path to the rulefile to use. This is determined by the RuleFile
	// function.
	rulefile string

	// the directory to use as the working directory when executing the
	// build file.
	dir string

	// The working directory.
	wd string

	stdout, stderr io.Writer
}

// newTarget initializes and returns a new target instance.
func newTarget(wd, name string) *target {
	path := abs(wd, name)

	rulefile := RuleFile(path)

	var dir string
	if rulefile != "" {
		dir = filepath.Dir(path)
	}

	return &target{
		name:     name,
		path:     path,
		rulefile: rulefile,
		dir:      dir,
		wd:       wd,
	}
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
		//
		// TODO(ejholmes): Do we really want to do this? There may be
		// cases where we don't actually need to verify that the file
		// exists on disk...
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.ruleCommand(ctx, PhaseExec)
	return cmd.Run()
}

// Dependencies executes the rule with "deps" as the first argument, and parses
// out the newline delimited list of dependencies.
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

	var deps []string
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}
		// Make all paths relative to the working directory.
		path, err := filepath.Rel(t.wd, filepath.Join(t.dir, scanner.Text()))
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
	pre := ansi("36", fmt.Sprintf("%s\t", t.name))
	cmd.Stdout = prefix(t.stdout, pre)
	cmd.Stderr = prefix(t.stderr, pre)
	cmd.Dir = t.dir
	return cmd
}

// verboseTarget simply wraps a target to print to to stdout when it's Exec'd.
type verboseTarget struct {
	*target
}

func (t *verboseTarget) Exec(ctx context.Context) error {
	err := t.target.Exec(ctx)
	if err != nil {
		return &targetError{t.target, err}
	}
	if err == nil && t.rulefile != "" {
		fmt.Printf("%s\n", ansi("32", "%s", t.Name()))
	}
	return err
}

// RuleFile is used to determine the path to an executable which will be used as
// the Rule to execute the given target. Given a target `hello.o`, this will try
// the following paths (relative to hello.o):
//
//	.walk/hello.o
//	hello.o.walk
//	.walk/default.o
//	default.o.walk
func RuleFile(path string) string {
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
			return path
		}
	}

	return ""
}

func abs(wd, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}
	return filepath.Clean(path)
}

// prefixWriter wraps an io.Writer to append a prefix to each line written.
type prefixWriter struct {
	prefix []byte

	// The underlying io.Writer where prefixed lines will be written.
	w io.Writer

	// Buffer to hold the last line, which doesn't have a newline yet.
	b []byte
}

func prefix(w io.Writer, prefix string) io.Writer {
	if w == nil {
		return w
	}
	return &prefixWriter{
		prefix: []byte(prefix),
		w:      w,
	}
}

func (w *prefixWriter) Write(b []byte) (int, error) {
	p := b
	for {
		i := bytes.IndexRune(p, '\n')

		if i >= 0 {
			w.b = append(w.b, p[:i+1]...)
			p = p[i+1:]
			_, err := w.w.Write(append(w.prefix, w.b...))
			w.b = nil
			if err != nil {
				return len(b), err
			}
			continue
		}

		w.b = append(w.b, p...)
		break
	}
	return len(b), nil
}

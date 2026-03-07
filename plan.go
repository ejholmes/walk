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

// Walkfile is the name of the file that will be executed to plan and build
// targets.
const Walkfile = "Walkfile"

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

// TargetOptions are options passed to the NewTarget factory method.
type TargetOptions struct {
	// The working directory that the target is relative to. The zero value
	// is to use os.Getwd().
	WorkingDir string

	// Stdout/Stderr streams.
	Stdout, Stderr io.Writer

	// If true, the stdout from the targets will be attached to the Stdout
	// provided above.
	Verbose bool

	// If true, disables prefixing of stdout/stderr
	NoPrefix bool
}

// NewTarget returns a new Target instance.
func NewTarget(options TargetOptions) func(string) (Target, error) {
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}

	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}

	var err error
	if options.WorkingDir == "" {
		options.WorkingDir, err = os.Getwd()
	}

	return func(name string) (Target, error) {
		if err != nil {
			return nil, err
		}

		t := newTarget(options.WorkingDir, name)
		if options.Verbose {
			if options.NoPrefix {
				t.stdout = options.Stdout
			} else {
				t.stdout = prefix(options.Stdout, t)
			}
		}
		if options.NoPrefix {
			t.stderr = options.Stderr
		} else {
			t.stderr = prefix(options.Stderr, t)
		}
		return &verboseTarget{
			target: t,
			stdout: options.Stdout,
		}, nil
	}
}

// Plan is used to build a graph of all the targets and their dependencies. It
// offers two primary methods; `Plan` and `Exec`, which correspond to the `deps`
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
		NewTarget: NewTarget(TargetOptions{}),
		graph:     newGraph(),
	}
}

// String implements the fmt.Stringer interface for Plan, which simply prints
// the targets and their dependencies.
func (p *Plan) String() string {
	return p.graph.String()
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
// Rule. Targets Exec functions are guaranteed to be called when all of the
// Targets dependencies have been fulfilled.
func (p *Plan) Exec(ctx context.Context, semaphore Semaphore) error {
	return p.graph.Walk(func(t Target) error {
		semaphore.P()
		defer semaphore.V()
		return t.Exec(ctx)
	})
}

// targetError is an error implementation that provides additional information
// about the rule that was used to build the target (if any).
type targetError struct {
	target *target
	err    error
}

func (e *targetError) Error() string {
	return fmt.Sprintf("%s: %v", e.target.Name(), e.err)
}

// ExitCodeFallback is the exit code that signals walk to try the next
// Walkfile in the inheritance chain.
const ExitCodeFallback = 127

// target is a Target implementation, that represents a file on disk, which may
// be built by a rule.
type target struct {
	// Relative path to the file.
	name string

	// The absolute path to the file.
	path string

	// rulefiles is the list of candidate Walkfiles, ordered from most
	// specific (closest to target) to least specific (furthest up the tree).
	rulefiles []string

	// rulefileIdx is the index into rulefiles of the active Walkfile.
	// This is set during Dependencies() when a Walkfile successfully handles
	// the target (doesn't return ExitCodeFallback).
	rulefileIdx int

	// The working directory.
	wd string

	stdout, stderr io.Writer
}

// newTarget initializes and returns a new target instance.
func newTarget(wd, name string) *target {
	path := abs(wd, name)
	rulefiles := RuleFiles(path)

	return &target{
		name:      name,
		path:      path,
		rulefiles: rulefiles,
		wd:        wd,
	}
}

// rulefile returns the currently active Walkfile path.
func (t *target) rulefile() string {
	if len(t.rulefiles) == 0 {
		return ""
	}
	return t.rulefiles[t.rulefileIdx]
}

// dir returns the working directory for the active Walkfile.
func (t *target) dir() string {
	if rf := t.rulefile(); rf != "" {
		return filepath.Dir(rf)
	}
	return ""
}

// targetName returns the target name relative to the active Walkfile's directory.
func (t *target) targetName() string {
	if dir := t.dir(); dir != "" {
		rel, _ := filepath.Rel(dir, t.path)
		return rel
	}
	return ""
}

// Name implements the Target interface.
func (t *target) Name() string {
	return t.name
}

// Exec executes the rule with "exec" as the first argument.
func (t *target) Exec(ctx context.Context) error {
	// No .walk file, meaning it's a static dependency.
	if t.rulefile() == "" {
		return nil
	}

	cmd, err := t.ruleCommand(ctx, PhaseExec)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// Dependencies executes the rule with "deps" as the first argument, and parses
// out the newline delimited list of dependencies.
func (t *target) Dependencies(ctx context.Context) ([]string, error) {
	// No .walk file, meaning it's a static dependency.
	if len(t.rulefiles) == 0 {
		return nil, nil
	}

	// Try each Walkfile in order until one handles the target
	// (doesn't return ExitCodeFallback).
	var lastErr error
	for i := range t.rulefiles {
		t.rulefileIdx = i

		b := new(bytes.Buffer)
		cmd, err := t.ruleCommand(ctx, PhaseDeps)
		if err != nil {
			return nil, err
		}
		cmd.Stdout = b

		if err := cmd.Run(); err != nil {
			// Check if this is a fallback signal
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == ExitCodeFallback {
				lastErr = err
				continue // Try next Walkfile
			}
			return nil, err
		}

		// This Walkfile handled it - parse dependencies
		var deps []string
		scanner := bufio.NewScanner(b)
		for scanner.Scan() {
			path := scanner.Text()
			if path == "" {
				continue
			}

			// If the path is not already and absolute path, make it one.
			if !filepath.IsAbs(path) {
				path = filepath.Join(t.dir(), path)
			}

			// Make all paths relative to the working directory.
			path, err := filepath.Rel(t.wd, path)
			if err != nil {
				return deps, err
			}
			deps = append(deps, path)
		}

		return deps, scanner.Err()
	}

	// All Walkfiles returned fallback - return the last error
	return nil, lastErr
}

func (t *target) ruleCommand(ctx context.Context, phase string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, t.rulefile(), phase, t.targetName())
	cmd.Stdout = t.stdout
	cmd.Stderr = t.stderr
	cmd.Dir = t.dir()
	return cmd, nil
}

// verboseTarget simply wraps a target to print to to stdout when it's Exec'd.
type verboseTarget struct {
	*target
	stdout io.Writer
}

func (t *verboseTarget) Exec(ctx context.Context) error {
	err := t.target.Exec(ctx)
	prefix := "ok"
	color := "32"
	if err != nil {
		prefix = "error"
		color = "31"
	}
	line := fmt.Sprintf("%s\t%s", ansi(color, "%s", prefix), t.target.Name())
	if err != nil {
		line = fmt.Sprintf("%s\t%s", line, err)
	}
	if t.rulefile() != "" {
		fmt.Fprintf(t.stdout, "%s\n", line)
	}
	if err != nil {
		return &targetError{t.target, err}
	}
	return err
}

// RuleFile is used to determine the path to an executable which will be used as
// the Rule to execute the given target. It walks up the directory tree from the
// target's directory until it finds a Walkfile.
func RuleFile(path string) string {
	rulefiles := RuleFiles(path)
	if len(rulefiles) == 0 {
		return ""
	}
	return rulefiles[0]
}

// RuleFiles returns all candidate Walkfiles for the given target path, ordered
// from most specific (closest to target) to least specific (furthest ancestor).
func RuleFiles(path string) []string {
	var rulefiles []string
	dir := filepath.Dir(path)

	for {
		walkfile := filepath.Join(dir, Walkfile)
		if _, err := os.Stat(walkfile); err == nil {
			rulefiles = append(rulefiles, walkfile)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return rulefiles
}

// prefixWriter wraps an io.Writer to append a prefix to each line written.
type prefixWriter struct {
	prefix []byte

	// The underlying io.Writer where prefixed lines will be written.
	w io.Writer

	// Buffer to hold the last line, which doesn't have a newline yet.
	b []byte
}

func prefix(w io.Writer, t Target) io.Writer {
	if w == nil {
		return w
	}
	prefix := ansi("36", fmt.Sprintf("%s\t", t.Name()))
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

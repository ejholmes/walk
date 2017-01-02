package main

import (
	"bufio"
	"bytes"
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
func Exec(target string) error {
	plan := newPlan()
	err := plan.Plan(target)
	if err != nil {
		return err
	}
	return plan.Exec()
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
func (p *Plan) Plan(target string) error {
	_, err := p.addTarget(target)
	if err != nil {
		return err
	}

	return p.graph.Validate()
}

func (p *Plan) addTarget(target string) (Target, error) {
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
		dep, err := p.addTarget(d)
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
func (t *FileTarget) Exec() error {
	if t.walkfile == "" {
		// It's possible for a target to simply be a static file, in which case
		// we don't need to perform a build. We do however want to ensure that
		// it exists in this case.
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.ruleCommand(PhaseExec)
	return cmd.Run()
}

func (t *FileTarget) Dependencies() ([]string, error) {
	// No .walk file, meaning it's a static dependency.
	if t.walkfile == "" {
		return nil, nil
	}

	b := new(bytes.Buffer)
	cmd := t.ruleCommand(PhaseDeps)
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

func (t *FileTarget) ruleCommand(subcommand string) *exec.Cmd {
	name := filepath.Base(t.path)
	cmd := exec.Command(t.walkfile, subcommand, name)
	cmd.Stdout = prefix(t.stdout, fmt.Sprintf("%s\t", t.Name()))
	cmd.Stderr = prefix(t.stderr, fmt.Sprintf("%s\t", t.Name()))
	cmd.Dir = t.dir
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

type prefixWriter struct {
	prefix []byte
	w      io.Writer
	b      []byte
}

func prefix(w io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{
		prefix: []byte(prefix),
		w:      w,
	}
}

func (w *prefixWriter) Write(b []byte) (int, error) {
	p := b
	for {
		i := bytes.IndexByte(p, '\n')

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

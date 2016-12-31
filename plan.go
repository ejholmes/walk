package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// The two arguments that .build executables will be called with, in the "plan"
// or "build" phases.
const (
	ArgDeps  = "deps"
	ArgBuild = "build"
)

// Target represents an individual node in the dependency graph.
type Target struct {
	// Name is the name of the target.
	Name string
}

// Plan is used to build a graph of all the targets and their dependencies.
type Plan struct {
	BuildFunc        func(*Target) error
	DependenciesFunc func(*Target) ([]string, error)

	graph *Graph
}

func newPlan() *Plan {
	return &Plan{
		graph: newGraph(),
	}
}

// Plan builds the graph, starting with the given target.
func (p *Plan) Plan(target string) (*Target, error) {
	t := &Target{
		Name: target,
	}

	if t := p.graph.Add(t); t != nil {
		// If this target already exists in the graph, nothing to do.
		return t, nil
	}

	deps, err := p.DependenciesFunc(t)
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

// Build executes the plan.
func (p *Plan) Build() error {
	err := p.graph.Walk(func(t *Target) error {
		return p.BuildFunc(t)
	})
	return err
}

func Dependencies(target *Target) ([]string, error) {
	t, err := newFileTarget(target)
	if err != nil {
		return nil, err
	}

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

func VerboseBuild(target *Target) error {
	t, err := newFileTarget(target)
	if err != nil {
		return err
	}

	err = BuildFile(t)
	if err == nil && t.buildfile != "" {
		fmt.Printf("%s\n", t.Name)
	}
	return err
}

// BuildFile builds the fileTarget.
func BuildFile(t *fileTarget) error {
	if t.buildfile == "" {
		// It's possible for a target to simply be a static file, in which case
		// we don't need to perform a build. We do however want to ensure that
		// it exists in this case.
		_, err := os.Stat(t.path)
		return err
	}

	cmd := t.buildCommand(ArgBuild)
	return cmd.Run()
}

// fileTarget extends a Target that's represented by a file.
type fileTarget struct {
	*Target

	// The absolute path to the file.
	path string

	// path to the buildfile to use.
	buildfile string

	// the directory to use as the working directory when executing the
	// build file.
	buildir string
}

// newFileTarget initializes and returns a new fileTarget instance.
func newFileTarget(t *Target) (*fileTarget, error) {
	path, err := filepath.Abs(t.Name)
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

	return &fileTarget{
		Target:    t,
		path:      path,
		buildfile: buildfile,
		buildir:   buildir,
	}, nil
}

func (t *fileTarget) buildCommand(subcommand string) *exec.Cmd {
	ext := filepath.Ext(t.path)
	name := filepath.Base(t.path)
	nameWithoutExt := name[0 : len(name)-len(ext)]
	cmd := exec.Command(t.buildfile, subcommand, name, nameWithoutExt)
	cmd.Stderr = os.Stderr
	cmd.Dir = t.buildir
	return cmd
}

// buildFile returns the path to the .build file that should be used to build
// this target. If the target has no appropriate .build file, then "" is
// returned.
func buildFile(path string) (string, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	try := []string{
		filepath.Join(".build", name),                          // .build/hello.o
		fmt.Sprintf("%s.build", name),                          // hello.o.build
		filepath.Join(".build", fmt.Sprintf("default%s", ext)), // .build/default.o
		fmt.Sprintf("default%s.build", ext),                    // default.o.build
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

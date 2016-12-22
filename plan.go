package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ejholmes/redo/dag"
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

	graph *dag.AcyclicGraph
}

func newPlan() *Plan {
	var graph dag.AcyclicGraph
	return &Plan{
		graph: &graph,
	}
}

// Build builds the graph, starting with the given target.
func (p *Plan) Build(target string) (*Target, error) {
	t := &Target{
		Name: target,
	}
	p.graph.Add(t)

	deps, err := p.DependenciesFunc(t)
	if err != nil {
		return t, fmt.Errorf("error getting dependencies for %s: %v", target, err)
	}

	for _, d := range deps {
		dep, err := p.Build(d)
		if err != nil {
			return t, err
		}
		p.graph.Connect(dag.BasicEdge(t, dep))
	}

	return t, nil
}

func (p *Plan) Execute() error {
	err := p.graph.Walk(func(v dag.Vertex) error {
		t := v.(*Target)
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

	cmd := t.buildCommand("deps")
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
		path, err := filepath.Rel(wd, filepath.Join(filepath.Dir(t.buildfile), scanner.Text()))
		if err != nil {
			return deps, err
		}
		deps = append(deps, path)
	}

	return deps, scanner.Err()
}

func VerboseBuild(t *Target) error {
	err := Build(t)
	if err == nil {
		fmt.Printf("build  %s\n", t.Name)
	}
	return err
}

// Build builds the target.
func Build(target *Target) error {
	t, err := newFileTarget(target)
	if err != nil {
		return err
	}

	// It's possible for a target to simply be a static file, in which case
	// we don't need to perform a build. We do however want to ensure that
	// it exists in this case.
	static := t.buildfile == ""

	if !static {
		cmd := t.buildCommand()
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	_, err = os.Stat(t.path)
	if err != nil {
		// If the file is generated, it's ok for the build to not
		// produce an artifact, however, if the file is static, then we
		// still want to return an error.
		if _, ok := err.(*os.PathError); ok && !static {
			return nil
		}
		return err
	}

	return nil
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
		buildir = filepath.Dir(buildfile)
	}

	return &fileTarget{
		Target:    t,
		path:      path,
		buildfile: buildfile,
		buildir:   buildir,
	}, nil
}

func (t *fileTarget) buildCommand(arg ...string) *exec.Cmd {
	cmd := exec.Command(t.buildfile, arg...)
	cmd.Stderr = os.Stderr
	cmd.Dir = t.buildir
	return cmd
}

// buildFile returns the path to the .build file that should be used to build
// this target. If the target has no approriate .build file, then "" is
// returned.
func buildFile(path string) (string, error) {
	p := fmt.Sprintf("%s.build", path)
	_, err := os.Stat(p)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return "", nil
		}
	}
	return p, err
}

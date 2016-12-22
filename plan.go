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

func Dependencies(t *Target) ([]string, error) {
	fullpath, err := filepath.Abs(t.Name)
	if err != nil {
		return nil, err
	}
	buildfile := fmt.Sprintf("%s.build", fullpath)
	_, err = os.Stat(buildfile)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil, nil
		}
		return nil, err
	}
	buildir := filepath.Dir(buildfile)

	cmd := exec.Command(buildfile, "deps")
	cmd.Stderr = os.Stderr
	cmd.Dir = buildir
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
		path, err := filepath.Rel(wd, filepath.Join(filepath.Dir(buildfile), scanner.Text()))
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
func Build(t *Target) error {
	fullpath, err := filepath.Abs(t.Name)
	if err != nil {
		return err
	}
	// It's possible for a target to simply be a static file, in which case
	// we don't need to perform a build. We do however want to ensure that
	// it exists in this case.
	static := false
	buildfile := fmt.Sprintf("%s.build", fullpath)
	_, err = os.Stat(buildfile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			return err
		}
		static = true
	}

	if !static {
		cmd := exec.Command(buildfile)
		cmd.Stderr = os.Stderr
		cmd.Dir = filepath.Dir(buildfile)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	_, err = os.Stat(fullpath)
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

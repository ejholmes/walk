package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ejholmes/redo/dag"
)

// Target represents an individual node in the dependency graph.
type Target struct {
	// Name is the name of the target.
	Name string

	// Hash is a hash of all of the content that gets built (if any)
	Hash hash.Hash
}

// Plan is used to build a graph of all the targets and their dependencies.
type Plan struct {
	BuildFunc        func(*Target) error
	DependenciesFunc func(string) ([]string, error)

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

	deps, err := p.DependenciesFunc(target)
	if err != nil {
		return t, fmt.Errorf("error getting dependencies for %s: %v", target, err)
	}

	for _, dep := range deps {
		dept, err := p.Build(dep)
		if err != nil {
			return t, err
		}
		p.graph.Connect(dag.BasicEdge(t, dept))
	}

	return t, nil
}

func (p *Plan) Execute() error {
	err := p.graph.Walk(func(v dag.Vertex) error {
		t := v.(*Target)
		t.Hash = newHash()
		return p.BuildFunc(t)
	})
	return err
}

var newHash = sha1.New

func Dependencies(name string) ([]string, error) {
	fullpath, err := filepath.Abs(name)
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
	cmd := exec.Command(buildfile, "deps")
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(buildfile)
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
		if filepath.Dir(buildfile) != wd {
			p, err := filepath.Rel(wd, filepath.Join(filepath.Dir(buildfile), path))
			if err != nil {
				return deps, err
			}
			path = p
		}
		deps = append(deps, path)
	}

	return deps, scanner.Err()
}

func VerboseBuild(t *Target) error {
	err := Build(t)
	if err == nil {
		fmt.Printf("build  %s (%x)\n", t.Name, t.Hash.Sum(nil)[0:4])
	}
	return err
}

func Build(t *Target) error {
	fullpath, err := filepath.Abs(t.Name)
	if err != nil {
		return err
	}
	buildfile := fmt.Sprintf("%s.build", fullpath)
	_, err = os.Stat(buildfile)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		return err
	}

	cmd := exec.Command(buildfile)
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(buildfile)
	if err := cmd.Run(); err != nil {
		return err
	}

	_, err = os.Stat(fullpath)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		return err
	}
	f, err := os.Open(fullpath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(t.Hash, f); err != nil {
		return err
	}
	return nil
}

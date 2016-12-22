package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/ejholmes/redo/dag"
)

// Target represents an individual node in the dependency graph.
type Target struct {
	// Name is the name of the target.
	Name string

	// Hash is a hash of all of the content that gets built (if any)
	Hash *Hash
}

// Hashes stores the last hash for a target.
type Hashes interface {
	Put(string, *Hash) error
	Get(string) (*Hash, error)
}

// Hash is used for determining whether a target needs to be rebuilt or not.
type Hash struct {
	// The hash of the target itself.
	Hash []byte

	// The hash of all of the targets dependencies hashes.
	Deps []byte
}

// Sum returns a sum of both the Hash and the Deps hash.
func (h *Hash) Sum(p []byte) []byte {
	m := sha1.New()
	m.Write(h.Hash)
	m.Write(h.Deps)
	return m.Sum(nil)
}

// Plan is used to build a graph of all the targets and their dependencies.
type Plan struct {
	Hashes Hashes

	BuildFunc        func(*Target) error
	DependenciesFunc func(string) ([]string, error)

	graph *dag.AcyclicGraph
}

func newPlan() *Plan {
	var graph dag.AcyclicGraph
	return &Plan{
		graph:  &graph,
		Hashes: newHashes(),
	}
}

// Build builds the graph, starting with the given target.
func (p *Plan) Build(target string) (*Target, error) {
	t := &Target{
		Name: target,
		Hash: newHash(),
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

		depshash := sha1.New()
		deps := p.graph.DownEdges(v).List()
		for _, dep := range deps {
			depshash.Write(dep.(*Target).Hash.Sum(nil))
		}
		t.Hash.Deps = depshash.Sum(nil)

		// Get the last hash for this target.
		h, err := p.Hashes.Get(t.Name)
		if err != nil {
			return err
		}

		// If any of the dependencies have changed, re-build this
		// target.
		if h == nil || !reflect.DeepEqual(t.Hash.Deps, h.Deps) || len(deps) == 0 {
			err := p.BuildFunc(t)
			if err != nil {
				return err
			}
			return p.Hashes.Put(t.Name, t.Hash)
		}

		t.Hash = h
		return nil
	})
	return err
}

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

	build := true
	_, err = os.Stat(buildfile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			return err
		}
		build = false
	}

	if build {
		cmd := exec.Command(buildfile)
		cmd.Stderr = os.Stderr
		cmd.Dir = filepath.Dir(buildfile)
		if err := cmd.Run(); err != nil {
			return err
		}
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
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	t.Hash.Hash = h.Sum(nil)
	return nil
}

func newHash() *Hash {
	return &Hash{}
}

type hashes struct {
	mu sync.Mutex
	m  map[string]*Hash
}

func newHashes() *hashes {
	return &hashes{m: make(map[string]*Hash)}
}

func (s *hashes) Put(name string, h *Hash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = h
	return nil
}

func (s *hashes) Get(name string) (*Hash, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name], nil
}

func (s *hashes) UnmarshalJSON(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(b, &s.m)
}

func (s *hashes) MarshalJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(s.m)
}

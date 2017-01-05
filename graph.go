package main

import (
	"context"
	"sync"

	"github.com/ejholmes/walk/internal/dag"
)

// Graph wraps a graph of targets.
type Graph struct {
	mu  sync.Mutex
	m   map[string]Target
	dag *dag.AcyclicGraph
}

func newGraph() *Graph {
	return &Graph{
		m:   make(map[string]Target),
		dag: new(dag.AcyclicGraph),
	}
}

// Add adds the target to the graph unless it already exists in the graph. If
// a target with the given name already exists in the graph, that existing
// target is returned, otherwise nil
func (g *Graph) Add(target Target) Target {
	g.mu.Lock()
	defer g.mu.Unlock()

	if t := g.target(target.Name()); t != nil {
		return t
	}

	g.m[target.Name()] = target
	g.dag.Add(target.Name())
	return nil
}

// Connect connects the two targets together.
func (g *Graph) Connect(target, dependency Target) {
	g.dag.Connect(dag.BasicEdge(target.Name(), dependency.Name()))
}

// Returns the Target with the given name.
func (g *Graph) Target(name string) Target {
	g.mu.Lock()
	target := g.target(name)
	g.mu.Unlock()
	return target
}

// Walk wraps the underlying Walk function to coerce it to a Target first.
func (g *Graph) Walk(fn func(Target) error) error {
	return g.dag.Walk(func(v dag.Vertex) error {
		target := g.Target(v.(string))
		// We don't actually need to walk the root, since it's a psuedo
		// target.
		if _, ok := target.(*rootTarget); ok {
			return nil
		}
		return fn(target)
	})
}

func (g *Graph) Validate() error {
	return g.dag.Validate()
}

func (g *Graph) String() string {
	return g.dag.String()
}

func (g *Graph) target(name string) Target {
	return g.m[name]
}

// rootTarget is a psuedo target for the root of the graph.
type rootTarget struct {
	deps []string
}

func (t *rootTarget) Name() string {
	return "(root)"
}

func (t *rootTarget) Exec(_ context.Context) error {
	panic("unreachable")
}

func (t *rootTarget) Dependencies(_ context.Context) ([]string, error) {
	return t.deps, nil
}

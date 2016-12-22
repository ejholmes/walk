package main

import (
	"sync"

	"github.com/ejholmes/build/dag"
)

// Graph wraps a graph of targets.
type Graph struct {
	mu  sync.Mutex
	m   map[string]*Target
	dag *dag.AcyclicGraph
}

func newGraph() *Graph {
	return &Graph{
		m:   make(map[string]*Target),
		dag: new(dag.AcyclicGraph),
	}
}

// Add adds the target to the graph unless it already exists in the graph. If
// a target with the given name already exists in the graph, that existing
// target is returned, otherwise nil
func (g *Graph) Add(target *Target) *Target {
	g.mu.Lock()
	defer g.mu.Unlock()

	if t := g.target(target.Name); t != nil {
		return t
	}

	g.m[target.Name] = target
	g.dag.Add(target.Name)
	return nil
}

// Connect connects the two targets together.
func (g *Graph) Connect(source, target *Target) {
	g.dag.Connect(dag.BasicEdge(source.Name, target.Name))
}

// Returns the Target with the given name.
func (g *Graph) Target(name string) *Target {
	g.mu.Lock()
	target := g.target(name)
	g.mu.Unlock()
	return target
}

// Walk wraps the underlying Walk function to coerce it to a Target first.
func (g *Graph) Walk(fn func(*Target) error) error {
	return g.dag.Walk(func(v dag.Vertex) error {
		return fn(g.Target(v.(string)))
	})
}

func (g *Graph) String() string {
	return g.dag.String()
}

func (g *Graph) target(name string) *Target {
	return g.m[name]
}

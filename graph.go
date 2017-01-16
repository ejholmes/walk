package main

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ejholmes/walk/internal/dag"
)

// WalkError is returned when a target fails while walking the graph.
type WalkError struct {
	mu     sync.Mutex
	Errors map[string]error
}

func newWalkError() *WalkError {
	return &WalkError{Errors: make(map[string]error)}
}

func (e *WalkError) Error() string {
	return fmt.Sprintf("%d targets failed", len(e.Errors))
}

func (e *WalkError) Add(t Target, err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Errors[t.Name()] = err
}

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
	errors := newWalkError()
	err := g.dag.Walk(func(v dag.Vertex) error {
		target := g.Target(v.(string))
		// We don't actually need to walk the root, since it's a psuedo
		// target.
		if _, ok := target.(*rootTarget); ok {
			return nil
		}
		err := fn(target)
		errors.Add(target, err)
		return err
	})

	if err == dag.ErrWalk {
		return errors
	}

	return err
}

func (g *Graph) Dependencies(target string) ([]Target, error) {
	set, err := g.dag.Ancestors(target)
	if err != nil {
		return nil, err
	}
	var t []Target
	for _, v := range dag.AsVertexList(set) {
		t = append(t, g.target(v.(string)))
	}
	return t, nil
}

func (g *Graph) TransitiveReduction() {
	g.dag.TransitiveReduction()
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

func dot(w io.Writer, g *Graph) error {
	if _, err := io.WriteString(w, "digraph {\n"); err != nil {
		return err
	}
	for _, v := range g.dag.Vertices() {
		for _, dep := range dag.AsVertexList(g.dag.DownEdges(v)) {
			if _, err := fmt.Fprintf(w, "  \"%s\" -> \"%s\"\n", dag.VertexName(v), dag.VertexName(dep)); err != nil {
				return err
			}
		}
	}
	if _, err := io.WriteString(w, "}\n"); err != nil {
		return err
	}
	return nil
}

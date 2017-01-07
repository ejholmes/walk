package main

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGraph(t *testing.T) {
	g := newGraph()

	a := &testTarget{name: "a"}
	b := &testTarget{name: "b"}
	r := &rootTarget{}

	var wg sync.WaitGroup
	wg.Add(3)

	add := func(t Target) {
		defer wg.Done()
		g.Add(t)
	}

	// Catches data races with -race flag.
	go add(a)
	go add(b)
	go add(r)

	wg.Wait()

	g.Connect(b, a)
	g.Connect(r, b)

	var mu sync.Mutex
	var targets []string

	g.Walk(func(t Target) error {
		mu.Lock()
		defer mu.Unlock()
		targets = append(targets, t.Name())
		return nil
	})

	assert.Equal(t, []string{"a", "b"}, targets)
}

func TestGraph_Dependencies(t *testing.T) {
	g := newGraph()

	a := &testTarget{name: "a"}
	b := &testTarget{name: "b"}
	c := &testTarget{name: "c"}
	d := &testTarget{name: "d"}
	r := &rootTarget{}

	g.Add(a)
	g.Add(b)
	g.Add(c)
	g.Add(r)

	g.Connect(b, a)
	g.Connect(c, b)
	g.Connect(c, a)
	g.Connect(d, c)
	g.Connect(r, b)

	deps := func(target string) []string {
		d, err := g.Dependencies(target)
		if err != nil {
			t.Fatal(err)
		}
		return targetNames(d)
	}

	assert.Nil(t, deps("a"))
	assert.Equal(t, []string{"a"}, deps("b"))
	assert.Equal(t, []string{"a", "b"}, deps("c"))
	assert.Equal(t, []string{"a", "b", "c"}, deps("d"))
}

type testTarget struct {
	name string
}

func (t *testTarget) Name() string {
	return t.name
}

func (t *testTarget) Exec(_ context.Context) error {
	return nil
}

func (t *testTarget) Dependencies(_ context.Context) ([]string, error) {
	return nil, nil
}

func targetNames(targets []Target) []string {
	var n sort.StringSlice
	for _, d := range targets {
		n = append(n, d.Name())
	}
	sort.Sort(n)
	return n
}

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ejholmes/redo/dag"
)

type node struct {
	name string
	err  error
}

func (n *node) String() string {
	return n.name
}

func main() {
	var graph dag.AcyclicGraph

	if _, err := buildGraph(&graph, "all"); err != nil {
		log.Fatal(err)
	}

	err := graph.Walk(func(v dag.Vertex) error {
		if node, ok := v.(string); ok && node == "all" {
			return nil
		}

		node := v.(*node)

		return build(node)
	})
	if err != nil {
		log.Fatal(err)
	}
}

// buildGraph finds the dependencies for each target, adds them to the graph,
// and connects an edge to the parent, recursively.
func buildGraph(graph *dag.AcyclicGraph, name string) (*node, error) {
	n := &node{name: name}
	graph.Add(n)

	path, err := filepath.Abs(fmt.Sprintf("%s.build", name))
	if err != nil {
		return n, err
	}
	_, err = os.Stat(path)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return n, nil
		}
		return n, err
	}
	cmd := exec.Command(path, "deps")
	cmd.Dir = filepath.Dir(path)
	out, err := cmd.Output()
	if err != nil {
		return n, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		dep, err := buildGraph(graph, text)
		if err != nil {
			return n, err
		}
		graph.Connect(dag.BasicEdge(n, dep))
	}

	if err := scanner.Err(); err != nil {
		return n, err
	}

	return n, nil
}

func build(node *node) error {
	fmt.Printf("build  %s\n", node.name)
	if node.err != nil {
		return node.err
	}
	path, err := filepath.Abs(fmt.Sprintf("%s.build", node.name))
	if err != nil {
		return err
	}
	_, err = os.Stat(path)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		return err
	}

	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	node.err = cmd.Run()
	return node.err
}

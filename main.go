package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/ejholmes/redo/dag"
)

type node struct {
	name string
	err  error

	hash    hash.Hash
	dephash hash.Hash
}

func (n *node) String() string {
	return n.name
}

func main() {
	var graph dag.AcyclicGraph

	target := "all"
	if len(os.Args) >= 2 {
		target = os.Args[1]
	}

	if _, err := buildGraph(&graph, target); err != nil {
		log.Fatal(err)
	}

	err := graph.Walk(func(v dag.Vertex) error {
		if node, ok := v.(string); ok && node == "all" {
			return nil
		}

		n := v.(*node)

		// Calculate a new dephash from the hashes of the direct
		// decendents.
		dephash := sha1.New()
		for _, edge := range graph.UpEdges(v).List() {
			dephash.Write(edge.(*node).hash.Sum(nil))
		}

		// If any of the dependencies have changed, build the node.
		if !reflect.DeepEqual(dephash.Sum(nil), n.dephash.Sum(nil)) {
			return verboseBuild(n)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

// buildGraph finds the dependencies for each target, adds them to the graph,
// and connects an edge to the parent, recursively.
func buildGraph(graph *dag.AcyclicGraph, name string) (*node, error) {
	n := &node{
		name:    name,
		hash:    sha1.New(),
		dephash: sha1.New(),
	}
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

func verboseBuild(node *node) error {
	err := build(node)
	if err == nil {
		fmt.Printf(" build  %s (built %x)\n", node.name, node.hash.Sum(nil))
	}
	return err
}

func build(node *node) error {
	if node.err != nil {
		return node.err
	}
	fullpath, err := filepath.Abs(node.name)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s.build", fullpath)
	_, err = os.Stat(path)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		return err
	}

	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	cmd.Stderr = os.Stderr
	node.err = cmd.Run()
	if node.err != nil {
		return node.err
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
	if _, err := io.Copy(node.hash, f); err != nil {
		return err
	}
	return node.err
}

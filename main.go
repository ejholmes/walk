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

func main() {
	var graph dag.AcyclicGraph
	graph.Add("all")

	path, err := filepath.Abs("all.build")
	if err != nil {
		log.Fatal(err)
	}
	_, err = os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command(path, "deps")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		dep := scanner.Text()
		v := &node{name: dep}
		graph.Add(v)
		graph.Connect(dag.BasicEdge("all", v))
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	err = graph.Walk(func(v dag.Vertex) error {
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

func build(node *node) error {
	fmt.Printf("build  %s\n", node.name)
	if node.err != nil {
		return node.err
	}
	path, err := filepath.Abs(fmt.Sprintf("%s.build", node.name))
	if err != nil {
		return err
	}

	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	node.err = cmd.Run()
	return node.err
}

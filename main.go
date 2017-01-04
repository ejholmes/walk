package main

import (
	"fmt"
	"os"
)

const DefaultTarget = "all"

func main() {
	target := DefaultTarget
	if len(os.Args) >= 2 {
		target = os.Args[1]
	}

	plan := newPlan()

	must(plan.Plan(target))
	must(plan.Exec(ExecOptions{
		Stdout: nil,
		Stderr: os.Stderr,
	}))
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

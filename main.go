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

	_, err := plan.Plan(target)
	must(err)

	must(plan.Exec())
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
)

func main() {
	target := "all"
	if len(os.Args) >= 2 {
		target = os.Args[1]
	}

	plan := newPlan()
	plan.DependenciesFunc = Dependencies
	plan.BuildFunc = VerboseBuild

	_, err := plan.Plan(target)
	must(err)

	must(plan.Build())
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

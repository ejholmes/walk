package main

import (
	"log"
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

	if _, err := plan.Build(target); err != nil {
		log.Fatal(err)
	}

	if err := plan.Execute(); err != nil {
		log.Fatal(err)
	}

}

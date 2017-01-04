package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const DefaultTarget = "all"

func main() {
	var (
		verbose = flag.Bool("v", false, "Show stdout from rules")
	)
	flag.Parse()

	target := DefaultTarget
	args := flag.Args()
	if len(args) >= 1 {
		target = args[0]
	}

	plan := newPlan()
	plan.NewTarget = newTarget(stdout(*verbose), os.Stderr)

	must(plan.Plan(target))
	must(plan.Exec())
}

func stdout(verbose bool) io.Writer {
	if verbose {
		return os.Stdout
	}
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

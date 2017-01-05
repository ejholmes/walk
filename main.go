package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ejholmes/walk/internal/tty"
)

const DefaultTarget = "all"

var isTTY bool

func init() {
	isTTY = isTerminal(os.Stdout)
}

func main() {
	var (
		verbose  = flag.Bool("v", false, fmt.Sprintf("Show stdout from rules when executing the %s phase.", PhaseExec))
		onlyplan = flag.Bool("p", false, fmt.Sprintf("Only plan the execution and print the graph. Does not execute the %s phase.", PhaseExec))
	)
	flag.Parse()

	target := DefaultTarget
	args := flag.Args()
	if len(args) >= 1 {
		target = args[0]
	}

	plan := newPlan()
	plan.NewTarget = newTarget(stdout(*verbose), os.Stderr)

	ctx := context.Background()

	must(plan.Plan(ctx, target))
	if *onlyplan {
		fmt.Print(plan)
	} else {
		must(plan.Exec(ctx))
	}
}

func stdout(verbose bool) io.Writer {
	if verbose {
		return os.Stdout
	}
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ansi("31", "error: %v", err))
		os.Exit(1)
	}
}
func isTerminal(w io.Writer) bool {
	if w == nil {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return tty.IsTerminal(f.Fd())
}

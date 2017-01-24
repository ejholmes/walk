package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ejholmes/walk/internal/tty"
)

const (
	// DefaultTarget is the name of the target that is used when no target
	// is provided on the command line.
	DefaultTarget = "all"
)

// Maps a named format to a function that renders the graph.
var printGraph = map[string]func(io.Writer, *Graph) error{
	"dot": dot,
}

var isTTY bool

func init() {
	isTTY = isTerminal(os.Stdout)
}

func main() {
	flag.Usage = usage
	var (
		version     = flag.Bool("version", false, "Print the version of walk and exit.")
		verbose     = flag.Bool("v", false, fmt.Sprintf("Show stdout from rules when executing the %s phase.", PhaseExec))
		noprefix    = flag.Bool("noprefix", false, "Disables the prefixing of stdout/stderr from rules with the name of the target.")
		deps        = flag.Bool("d", false, "Print the dependencies of the target.")
		concurrency = flag.Uint("j", 0, "The number of targets that are executed in parallel.")
		print       = flag.String("p", "", "Print the graph that will be executed and exit.")
	)
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s\n", Version)
		os.Exit(0)
	}

	targets := flag.Args()
	if len(targets) == 0 {
		targets = []string{DefaultTarget}
	}
	if *deps && len(targets) > 1 {
		must(fmt.Errorf("only 1 target is allowed when using the -d flag."))
	}

	plan := newPlan()
	plan.NewTarget = NewTarget(TargetOptions{
		Verbose:  *verbose,
		NoPrefix: *noprefix,
	})

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-c
			cancel()
		}
	}()

	must(plan.Plan(ctx, targets...))
	if *deps {
		deps, err := plan.Dependencies(targets[0])
		must(err)
		for _, t := range deps {
			fmt.Println(t.Name())
		}
	} else if *print != "" {
		fn, ok := printGraph[*print]
		if !ok {
			must(fmt.Errorf("invalid format provided: %s", *print))
		}
		must(fn(os.Stdout, plan.graph))
	} else {
		semaphore := NewSemaphore(*concurrency)
		must(plan.Exec(ctx, semaphore))
	}
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

func usage() {
	fmt.Fprintf(os.Stderr, "walk - A fast, general purpose, graph based build and task execution utility.\n\n")
	fmt.Fprintf(os.Stderr, "VERSION:\n")
	fmt.Fprintf(os.Stderr, "   %s\n\n", Version)
	fmt.Fprintf(os.Stderr, "USAGE:\n")
	fmt.Fprintf(os.Stderr, "   walk [target...]\n\n")
	fmt.Fprintf(os.Stderr, "OPTIONS:\n")
	flag.PrintDefaults()
}

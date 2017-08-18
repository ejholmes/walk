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
	"dot":   dot,
	"plain": plain,
}

var isTTY bool

func init() {
	isTTY = isTerminal(os.Stdout)
}

func main() {
	flag.Usage = usage
	var (
		version     = flag.Bool("version", false, "Print the version of walk and exit.")
		verbose     = flag.Bool("v", false, fmt.Sprintf("Show stdout from the Walkfile when executing the %s phase.", PhaseExec))
		noprefix    = flag.Bool("noprefix", false, "By default, the stdout/stderr output from the Walkfile is prefixed with the name of the target, followed by a tab character. This flag disables the prefixing. This can help with performance, or issues where you encounter \"too many open files\", since prefixing necessitates more file descriptors.")
		concurrency = flag.Uint("j", 0, "Controls the number of targets that are executed in parallel. By default, targets are executed with the maximum level of parallelism that the graph allows. To limit the number of targets that are executed in parallel, set this to a value greater than 1. To execute targets serially, set this to 1.")
		print       = flag.String("p", "", "Prints the underlying DAG to stdout, using the provided format. Available formats are \"dot\" and \"plain\".")
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
	if *print != "" {
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

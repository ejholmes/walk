package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	var (
		pre = flag.String("prefix", "", "A string to prefix stdout/stderr with.")
	)
	flag.Parse()
	args := flag.Args()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = prefix(os.Stdout, *pre)
	cmd.Stderr = prefix(os.Stderr, *pre)
	cmd.Stdin = os.Stdin

	err := cmd.Start()
	must(err)

	ch := make(chan os.Signal)
	signal.Notify(ch)
	go func() {
		for sig := range ch {
			cmd.Process.Signal(sig)
		}
	}()

	err = cmd.Wait()
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.ProcessState.Sys().(syscall.WaitStatus); ok {
			os.Exit(status.ExitStatus())
		}
	}
	must(err)
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "err: %v\n", err)
		os.Exit(1)
	}
}

// prefixWriter wraps an io.Writer to append a prefix to each line written.
type prefixWriter struct {
	prefix []byte

	// The underlying io.Writer where prefixed lines will be written.
	w io.Writer

	// Buffer to hold the last line, which doesn't have a newline yet.
	b []byte
}

func prefix(w io.Writer, prefix string) io.Writer {
	if w == nil {
		return w
	}
	return &prefixWriter{
		prefix: []byte(prefix),
		w:      w,
	}
}

func (w *prefixWriter) Write(b []byte) (int, error) {
	p := b
	for {
		i := bytes.IndexRune(p, '\n')

		if i >= 0 {
			w.b = append(w.b, p[:i+1]...)
			p = p[i+1:]
			_, err := w.w.Write(append(w.prefix, w.b...))
			w.b = nil
			if err != nil {
				return len(b), err
			}
			continue
		}

		w.b = append(w.b, p...)
		break
	}
	return len(b), nil
}

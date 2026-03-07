package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ejholmes/walk/internal/dag"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestPlan(t *testing.T) {
	clean(t)

	err := Exec(ctx, NewSemaphore(0), "test/110-compile/all")
	assert.NoError(t, err)
}

func TestPlan_Multi(t *testing.T) {
	clean(t)

	err := Exec(ctx, NewSemaphore(0), "test/110-compile/all", "test/111-compile/all")
	assert.NoError(t, err)
}

func TestPlan_CyclicDependencies(t *testing.T) {
	clean(t)

	err := Exec(ctx, NewSemaphore(0), "test/000-cyclic/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors[0].Error(), "Cycle"))
}

func TestPlan_Cancel(t *testing.T) {
	clean(t)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	err := Exec(ctx, NewSemaphore(0), "test/000-cancel/all").(*WalkError)
	assert.Equal(t, 2, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors["test/000-cancel/b.sleep"].Error(), "signal: killed"))
	assert.True(t, strings.Contains(err.Errors["test/000-cancel/a.sleep"].Error(), "signal: killed"))
}

func TestTarget_Dependencies(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	target := newTarget(wd, "test/110-compile/all")

	deps, err := target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"test/110-compile/hello", "test/110-compile/test"}, deps)

	target.wd = filepath.Join(target.wd, "test")
	deps, err = target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"110-compile/hello", "110-compile/test"}, deps)
}

func TestTarget_Dependencies_EmptyTarget(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	target := newTarget(wd, "test/000-empty-dependency/all")

	deps, err := target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"test/000-empty-dependency/a", "test/000-empty-dependency/b"}, deps)
}

func TestPlan_Error(t *testing.T) {
	clean(t)

	b := new(bytes.Buffer)
	plan := newPlan()
	plan.NewTarget = NewTarget(TargetOptions{
		Stdout: b,
	})
	err := plan.Plan(ctx, "test/000-cancel/fail")
	assert.NoError(t, err)

	err = plan.Exec(ctx, NewSemaphore(0))
	assert.Error(t, err)

	assert.Equal(t, "error\ttest/000-cancel/fail\texit status 1\n", b.String())
}

func TestPlan_NoWalkfile(t *testing.T) {
	// Use a temp directory that truly has no Walkfile anywhere in its ancestry
	tmpdir := t.TempDir()
	targetPath := filepath.Join(tmpdir, "all")

	b := new(bytes.Buffer)
	plan := newPlan()
	plan.NewTarget = NewTarget(TargetOptions{
		WorkingDir: tmpdir,
		Stdout:     b,
	})
	err := plan.Plan(ctx, "all")
	assert.NoError(t, err)

	err = plan.Exec(ctx, NewSemaphore(0))
	assert.NoError(t, err)

	// If there's no Walkfile in the directory (or any parent), it's a static
	// file. We don't really need to show these in output.
	assert.Equal(t, "", b.String())

	_ = targetPath // silence unused warning
}

func TestPrefixWriter(t *testing.T) {
	b := new(bytes.Buffer)
	w := &prefixWriter{w: b, prefix: []byte("prefix: ")}

	// Lines are buffered until a newline.
	w.Write([]byte("foo\n"))
	assert.Equal(t, "prefix: foo\n", b.String())

	// When a newline appears, line is prefixed and flushed.
	w.Write([]byte("bar"))
	assert.Equal(t, "prefix: foo\n", b.String())
	w.Write([]byte("\n"))
	assert.Equal(t, "prefix: foo\nprefix: bar\n", b.String())

	b.Reset()
	w.b = nil
	io.Copy(w, strings.NewReader(`I stand and listen, head bowed, 
to my inner complaint. 
Persons passing by think 
I am searching for a lost coin. 
You’re fired, I yell inside 
after an especially bad episode. 
I’m letting you go without notice 
or terminal pay. You just lost 
another chance to make good. 
But then I watch myself standing at the exit, 
depressed and about to leave, 
and wave myself back in wearily, 
for who else could I get in my place 
to do the job in dark, airless conditions?
`))

	assert.Equal(t, `prefix: I stand and listen, head bowed, 
prefix: to my inner complaint. 
prefix: Persons passing by think 
prefix: I am searching for a lost coin. 
prefix: You’re fired, I yell inside 
prefix: after an especially bad episode. 
prefix: I’m letting you go without notice 
prefix: or terminal pay. You just lost 
prefix: another chance to make good. 
prefix: But then I watch myself standing at the exit, 
prefix: depressed and about to leave, 
prefix: and wave myself back in wearily, 
prefix: for who else could I get in my place 
prefix: to do the job in dark, airless conditions?
`, b.String())
}

func TestRuleFile_Inheritance(t *testing.T) {
	// Create a temp directory structure:
	// tmpdir/
	//   Walkfile        <- should be found
	//   subdir/
	//     target.txt    <- target here, no local Walkfile
	tmpdir := t.TempDir()

	walkfile := filepath.Join(tmpdir, "Walkfile")
	err := os.WriteFile(walkfile, []byte("#!/bin/bash\necho test"), 0755)
	assert.NoError(t, err)

	subdir := filepath.Join(tmpdir, "subdir")
	err = os.Mkdir(subdir, 0755)
	assert.NoError(t, err)

	targetPath := filepath.Join(subdir, "target.txt")

	// RuleFile should find the Walkfile in the parent directory
	found := RuleFile(targetPath)
	assert.Equal(t, walkfile, found)
}

func TestRuleFile_Inheritance_EndToEnd(t *testing.T) {
	// Create a temp directory structure with a Walkfile that handles subdirectory targets
	tmpdir := t.TempDir()

	// Walkfile that echoes the target name for deps and creates a marker file on exec
	walkfile := filepath.Join(tmpdir, "Walkfile")
	err := os.WriteFile(walkfile, []byte(`#!/bin/bash
phase=$1
target=$2

case $phase in
  deps) ;; # no deps
  exec) touch "$target.built" ;;
esac
`), 0755)
	assert.NoError(t, err)

	// Create a subdirectory (no Walkfile here - should inherit)
	subdir := filepath.Join(tmpdir, "subdir")
	err = os.Mkdir(subdir, 0755)
	assert.NoError(t, err)

	b := new(bytes.Buffer)
	plan := newPlan()
	plan.NewTarget = NewTarget(TargetOptions{
		WorkingDir: tmpdir,
		Stdout:     b,
	})

	// Build a target in the subdirectory
	err = plan.Plan(ctx, "subdir/foo")
	assert.NoError(t, err)

	err = plan.Exec(ctx, NewSemaphore(0))
	assert.NoError(t, err)

	// Verify the Walkfile was invoked and created the marker file
	_, err = os.Stat(filepath.Join(tmpdir, "subdir/foo.built"))
	assert.NoError(t, err, "expected subdir/foo.built to exist")
}

func TestRuleFile_LocalOverridesParent(t *testing.T) {
	// Create a temp directory structure:
	// tmpdir/
	//   Walkfile        <- parent Walkfile
	//   subdir/
	//     Walkfile      <- local Walkfile (should be used)
	//     target.txt
	tmpdir := t.TempDir()

	parentWalkfile := filepath.Join(tmpdir, "Walkfile")
	err := os.WriteFile(parentWalkfile, []byte("#!/bin/bash\necho parent"), 0755)
	assert.NoError(t, err)

	subdir := filepath.Join(tmpdir, "subdir")
	err = os.Mkdir(subdir, 0755)
	assert.NoError(t, err)

	localWalkfile := filepath.Join(subdir, "Walkfile")
	err = os.WriteFile(localWalkfile, []byte("#!/bin/bash\necho local"), 0755)
	assert.NoError(t, err)

	targetPath := filepath.Join(subdir, "target.txt")

	// RuleFile should find the local Walkfile first
	found := RuleFile(targetPath)
	assert.Equal(t, localWalkfile, found)
}

func clean(t testing.TB) {
	err := Exec(ctx, NewSemaphore(0), "test/clean")
	assert.NoError(t, err)
}

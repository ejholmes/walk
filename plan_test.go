package main

import (
	"bytes"
	"context"
	"fmt"
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
	err := Exec(ctx, NewSemaphore(0), "test/000-cancel/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors[0].Error(), "signal: killed"))
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

func TestRuleFile(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	tests := []struct {
		target string
		rule   string
	}{
		{abs(wd, "test/110-compile/hello.o"), abs(wd, "test/110-compile/hello.o.walk")},
		{"test/111-compile/hello.o", "test/111-compile/default.o.walk"},
		{"test", ".walk/test"},
		{"foo", ""},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d: %s", i, tt.target), func(t *testing.T) {
			assert.Equal(t, tt.rule, RuleFile(tt.target))
		})
	}
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

func clean(t testing.TB) {
	err := Exec(ctx, NewSemaphore(0), "test/clean")
	assert.NoError(t, err)
}

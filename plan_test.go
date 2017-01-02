package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ejholmes/walk/internal/dag"
	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	clean(t)

	err := Exec("test/110-compile/all")
	assert.NoError(t, err)
}

func TestPlan_CyclicDependencies(t *testing.T) {
	clean(t)

	err := Exec("test/000-cyclic/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors[0].Error(), "Cycle"))
}

func clean(t testing.TB) {
	err := Exec("test/clean")
	assert.NoError(t, err)
}

func TestPrefixWriter(t *testing.T) {
	w := new(bytes.Buffer)
	pw := &prefixWriter{w: w, prefix: []byte("test  ")}

	io.WriteString(pw, `hello

world
`)

	assert.Equal(t, `test  hello
test  
test  world
`, w.String())
}

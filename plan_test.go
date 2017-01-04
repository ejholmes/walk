package main

import (
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
	assert.EqualError(t, err.Errors[0], "Cycle: test/000-cyclic/b, test/000-cyclic/a")
}

func clean(t testing.TB) {
	err := Exec("test/clean")
	assert.NoError(t, err)
}

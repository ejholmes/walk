package main

import (
	"os"
	"testing"

	"github.com/ejholmes/walk/internal/dag"
	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	err := testExec("test/clean")
	assert.NoError(t, err)

	err = testExec("test/110-compile/all")
	assert.NoError(t, err)
}

func TestPlan_CyclicDependencies(t *testing.T) {
	err := testExec("test/000-cyclic/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.EqualError(t, err.Errors[0], "Cycle: test/000-cyclic/b, test/000-cyclic/a")
}

func testExec(target string) error {
	return Exec(ExecOptions{Stderr: os.Stderr}, target)
}

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ejholmes/walk/internal/dag"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func TestPlan(t *testing.T) {
	clean(t)

	err := Exec(ctx, "test/110-compile/all")
	assert.NoError(t, err)
}

func TestPlan_CyclicDependencies(t *testing.T) {
	clean(t)

	err := Exec(ctx, "test/000-cyclic/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors[0].Error(), "Cycle"))
}

func TestPlan_Cancel(t *testing.T) {
	clean(t)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	err := Exec(ctx, "test/000-cancel/all").(*dag.MultiError)
	assert.Equal(t, 1, len(err.Errors))
	assert.True(t, strings.Contains(err.Errors[0].Error(), "signal: killed"))
}

func clean(t testing.TB) {
	err := Exec(ctx, "test/clean")
	assert.NoError(t, err)
}

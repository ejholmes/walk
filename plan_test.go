package main

import (
	"context"
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
	target, err := newTarget("test/110-compile/all")
	assert.NoError(t, err)

	deps, err := target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"test/110-compile/hello", "test/110-compile/test"}, deps)

	target.wd = filepath.Join(target.wd, "test")
	deps, err = target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"110-compile/hello", "110-compile/test"}, deps)
}

func TestTarget_Dependencies_EmptyTarget(t *testing.T) {
	target, err := newTarget("test/000-empty-dependency/all")
	assert.NoError(t, err)

	deps, err := target.Dependencies(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"test/000-empty-dependency/a", "test/000-empty-dependency/b"}, deps)
}

func clean(t testing.TB) {
	err := Exec(ctx, NewSemaphore(0), "test/clean")
	assert.NoError(t, err)
}

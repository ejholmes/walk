package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testTarget struct {
	name string
}

func (t *testTarget) Dependencies() ([]string, error) {
	switch t.name {
	case "all":
		return []string{
			"b/all",
		}, nil
	case "b/all":
		return []string{
			"b/hello",
		}, nil
	case "b/hello":
		return []string{
			"b/hello.c",
		}, nil
	case "b/hello.c":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown %s", t.name)
	}
}

func (t *testTarget) Exec() error {
	switch t.name {
	case "b/hello.c":
		return nil
	case "b/hello":
		return nil
	case "b/all":
		return nil
	case "a/all":
		return nil
	}
	return nil
}

func (t *testTarget) Name() string {
	return t.name
}

func newTestTarget(name string) (Target, error) {
	return &testTarget{name}, nil
}

func TestPlan(t *testing.T) {
	plan := newPlan()
	plan.NewTarget = newTestTarget

	_, err := plan.Plan("all")
	assert.NoError(t, err)

	err = plan.Exec()
	assert.NoError(t, err)
}

func logVisit(t *testing.T, f func(Target) error) func(Target) error {
	return func(target Target) error {
		err := f(target)
		t.Logf("Visited %s", target.Name())
		return err
	}
}

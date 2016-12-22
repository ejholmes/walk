package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	plan := newPlan()
	plan.DependenciesFunc = func(t *Target) ([]string, error) {
		switch t.Name {
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
			return nil, fmt.Errorf("unknown %s", t.Name)
		}
	}
	plan.BuildFunc = logVisit(t, func(t *Target) error {
		switch t.Name {
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
	})

	_, err := plan.Build("all")
	assert.NoError(t, err)

	err = plan.Execute()
	assert.NoError(t, err)
}

func logVisit(t *testing.T, f func(*Target) error) func(*Target) error {
	return func(target *Target) error {
		err := f(target)
		t.Logf("Visited %s", target.Name)
		return err
	}
}

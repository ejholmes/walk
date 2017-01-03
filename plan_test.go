package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	err := Exec("test/clean")
	assert.NoError(t, err)

	err = Exec("test/110-compile/all")
	assert.NoError(t, err)
}

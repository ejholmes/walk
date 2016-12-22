package main

import (
	"crypto/sha1"
	"fmt"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	plan := newPlan()
	plan.DependenciesFunc = func(name string) ([]string, error) {
		switch name {
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
			return nil, fmt.Errorf("unknown %s", name)
		}
	}

	var built []string
	plan.BuildFunc = logVisit(t, func(t *Target) error {
		built = append(built, t.Name)
		switch t.Name {
		case "b/hello.c":
			h := sha1Hash([]byte("int main"))
			t.Hash.Hash = h.Sum(nil)
			return nil
		case "b/hello":
			h := sha1Hash([]byte("some binary"))
			t.Hash.Hash = h.Sum(nil)
			return nil
		case "b/all":
			return nil
		case "all":
			return nil
		default:
			return fmt.Errorf("unknown %s", t.Name)
		}
	})

	hellocHash := &Hash{
		Hash: sha1Hash([]byte("int main")).Sum(nil),
		Deps: sha1.New().Sum(nil),
	}
	helloHash := &Hash{
		Hash: sha1Hash([]byte("some binary")).Sum(nil),
		Deps: sha1.New().Sum(nil),
	}
	d := sha1.New()
	d.Write(hellocHash.Sum(nil))
	helloHash.Deps = d.Sum(nil)
	plan.Hashes.Put("b/hello.c", hellocHash)
	plan.Hashes.Put("b/hello", helloHash)

	_, err := plan.Build("all")
	assert.NoError(t, err)

	err = plan.Execute()
	assert.NoError(t, err)
	assert.Equal(t, []string{"b/hello.c", "b/all", "all"}, built)
}

func TestPlan_BustCache(t *testing.T) {
	plan := newPlan()
	plan.DependenciesFunc = func(name string) ([]string, error) {
		switch name {
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
			return nil, fmt.Errorf("unknown %s", name)
		}
	}

	var built []string
	plan.BuildFunc = logVisit(t, func(t *Target) error {
		built = append(built, t.Name)
		switch t.Name {
		case "b/hello.c":
			h := sha1Hash([]byte("int main()"))
			t.Hash.Hash = h.Sum(nil)
			return nil
		case "b/hello":
			h := sha1Hash([]byte("some binary"))
			t.Hash.Hash = h.Sum(nil)
			return nil
		case "b/all":
			return nil
		case "all":
			return nil
		default:
			return fmt.Errorf("unknown %s", t.Name)
		}
	})

	hellocHash := &Hash{
		Hash: sha1Hash([]byte("int main")).Sum(nil),
		Deps: sha1.New().Sum(nil),
	}
	helloHash := &Hash{
		Hash: sha1Hash([]byte("some binary")).Sum(nil),
		Deps: sha1.New().Sum(nil),
	}
	d := sha1.New()
	d.Write(hellocHash.Sum(nil))
	helloHash.Deps = d.Sum(nil)
	plan.Hashes.Put("b/hello.c", hellocHash)
	plan.Hashes.Put("b/hello", helloHash)

	_, err := plan.Build("all")
	assert.NoError(t, err)

	err = plan.Execute()
	assert.NoError(t, err)
	assert.Equal(t, []string{"b/hello.c", "b/hello", "b/all", "all"}, built)
}

func logVisit(t *testing.T, f func(*Target) error) func(*Target) error {
	return func(target *Target) error {
		err := f(target)
		t.Logf("Visited %s (%x)", target.Name, target.Hash.Sum(nil))
		return err
	}
}

func sha1Hash(b []byte) hash.Hash {
	h := sha1.New()
	h.Write(b)
	return h
}

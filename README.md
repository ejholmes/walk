A fast, universal build system, to build anything and everything.

Heavily inspired by `make` and [redo](https://github.com/apenwarr/redo).

## What can it build?

* Frontend applications (e.g. sass -> css -> compress).
* C++ applications.
* Infrastructure.
* Anything that has dependencies.

## Features

* Blazingly fast parallel builds.
* Build shared dependencies once.
* Only build targets if their dependencies changed.

## DAG

At the core of XXX is a Directed Acyclic Graph (DAG). DAG's are a magical data structure that allow you to easily express dependency trees. You'll find DAG's everywhere; in GIT, languages, infrastructure tools, etc. XXX tries to generalize build systems based on top of this solid primitive.

## Targets

Similar to make, XXX has the concept of "targets". A target is simply a file that can be built from some dependencies. However, unlike make (and like redo), targets are described as executable files instead of in a Makefile. This provides for unlimited flexibility to define how something is built, and what it depends on, by composing existing tools.

For example, if I wanted to describe how to build a binary called "hello" from "hello.c", I could do so with a bash script called "hello.build":


```bash
#!/bin/bash

deps="hello.c"

case $1 in
  deps)
    echo $deps ;;
  build)
    gcc -Wall -o hello $deps
esac
```

Executing `redo hello` will build the target:

```$
$ redo hello
build  hello
```

## Phases

XXX has two phases:

1. **Plan Phase**: In this phase, XXX executes all the `.build` files with `deps` as the first argument. `.build` files are expected to print a newline delimited list of files that the target depends on, relative to the `.build` file. Internally, XXX builds a graph of all of the targets and their dependencies.
2. **Build Phase**: In this phase, XXX executes all of the `.build` files with `build` as the first argument. `.build` files are expected to build the given target.

By separating these phases, XXX can build a compact dependency graph, and perform fast parallel builds.

# [Walk](http://ejholmes.io/walk/)

[![Build Status](https://travis-ci.org/ejholmes/walk.svg?branch=master)](https://travis-ci.org/ejholmes/walk)
[![Go Report Card](https://goreportcard.com/badge/github.com/ejholmes/walk)](https://goreportcard.com/report/github.com/ejholmes/walk)
[![Latest Version](https://img.shields.io/github/release/ejholmes/walk.svg?style=flat?maxAge=86400)](https://github.com/ejholmes/walk/releases)

`walk` is a fast, general purpose, graph based build and task execution utility.

Heavily inspired by [make](https://www.gnu.org/software/make/) and [redo](https://github.com/apenwarr/redo).

![](./docs/walk.gif)

## Features

* Fast parallel execution.
* Graph based dependency management.
* Maximum composability with existing UNIX tooling.
* Describe targets and their dependencies as simple executables.
* Universal execution; execute `walk` from any directory.

## Installation

Using Go 1.7+:

```console
$ go get -u github.com/ejholmes/walk
```

Or grab the latest release from https://github.com/ejholmes/walk/releases.

**NOTE**: `walk` is currently not considered stable, and there may be breaking changes before the 1.0 release.

## Usage

`walk` is built on top of a very simple concept; when you want to build a target, `walk` executes a file called `Walkfile` to determine:

1. What other targets the given target depends on.
2. How to build the target.

For example, if you wanted to build a program called `prog` from [main.c](./test/113-readme/main.c) and [parse.c](./test/113-readme/parse.c), you might write a `Walkfile` like this:

```bash
#!/bin/bash

# The first argument is the "phase", which will either be `deps` or `exec`. In
# the `deps` phase, the Walkfile should print the name of the targets that this
# target depends on.
phase=$1

# The second argument is the name of the target, like `prog`, `parse.o`, etc.
target=$2

case $target in
  prog)
    case $phase in
      # Prog depends on the object files we'll build from source. We simply
      # print each dependency on a single line.
      deps)
        echo main.o
        echo parse.o
        ;;
      exec) exec gcc -Wall -o $target $($0 deps $target) ;;
    esac ;;

  # A generic recipe for building a .o file from a corresponding .c file.
  *.o)
    case $phase in
      deps) echo ${target//.o/.c} ;;
      exec) exec gcc -Wall -o $target -c $($0 deps $target) ;;
    esac ;;

  # When invoking walk(1) without any arguments, it defaults to a target called
  # `all`.
  all)
    case $phase in
      deps) echo prog ;;
    esac ;;

  # In general, it's good practice to include a fallback rule like this, in
  # case someone tries to build a target that we don't know how to build (or
  # someone makes a typo).
  *.c|*.h) ;; # static files
  *) >&2 echo "No rule for target \"$target\"" && exit 1 ;;
esac
```

When you execute `walk all`, the following happens internally:

1. `walk` resolves all of the dependencies, and builds a graph:

    ```console
    $ Walkfile deps all
    prog
    $ Walkfile deps prog
    parse.o
    main.o
    $ Walkfile deps parse.o
    parse.c
    parse.h
    $ Walkfile deps main.o
    main.c
    $ Walkfile deps parse.c
    $ Walkfile deps parse.h
    $ Walkfile deps main.c
    ```

2. `walk` executes all of the targets, starting with dependencies:

    ```console
    $ Walkfile exec parse.c
    $ Walkfile exec parse.h
    $ Walkfile exec main.c
    $ Walkfile exec main.o
    $ Walkfile exec parse.o
    $ Walkfile exec prog
    $ Walkfile exec all
    ```

Ultimately, all of our targets end up getting invoked, and `prog` is built:

```console
$ walk
ok	main.c
ok	parse.c
ok	parse.o
ok	main.o
ok	prog
ok	all
```

`walk` always executes a Walkfile in the same directory as the target. So if you specified a target name like `src/hello.o`, then `walk` will execute a Walkfile in the `src` directory. This allows for uniform execution, so that `walk` can be executed from any directory, and always get the same result. This also makes it easy to build up increasingly complex build systems, by composing targets in subdirectories, which is very difficult to do with make(1).

And that's basically all you need to know about `walk`. Walkfile's can be written in any language you want, as long as they adhere to this very simple contract. No complicated, restrictive syntax to learn!

See also [`man walk`](http://ejholmes.io/walk/).

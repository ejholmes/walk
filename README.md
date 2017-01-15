# [Walk](http://ejholmes.io/walk/) [![Build Status](https://travis-ci.org/ejholmes/walk.svg?branch=master)](https://travis-ci.org/ejholmes/walk)

walk(1) is a fast, general purpose, graph based build and task execution utility.

Heavily inspired by [make](https://www.gnu.org/software/make/) and [redo](https://github.com/apenwarr/redo).

![](./docs/walk.gif)

## Features

* Fast parallel execution.
* Graph based dependency management.
* Maximum composability with existing UNIX tooling.
* Describe targets and their dependencies as simple executables.

## Installation

Using Go 1.7+:

```console
$ go get -u github.com/ejholmes/walk
```

Or grab the latest release from https://github.com/ejholmes/walk/releases.

## Usage

See [`man walk`](http://ejholmes.io/walk/).

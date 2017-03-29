walk(1) -- A fast, general purpose, graph based build and task execution utility.
==============================================================================================

## SYNOPSIS

`walk` `--help`<br>
`walk` [`-v`] [target...]<br>

## DESCRIPTION

walk(1) is a small utility that can be used to execute tasks, or build programs
from source. It's similar to make(1) in many ways, but with some fundamental
differences that make it vastly simpler, and arguably more powerful.

At the core of walk(1) is a [Directed Acyclic
Graph](https://en.wikipedia.org/wiki/Directed_acyclic_graph) (DAG). DAG's are a
magical data structure that allow you to easily express dependency trees.
You'll find DAG's everywhere; in
[git](http://eagain.net/articles/git-for-computer-scientists/), languages,
[infrastructure tools](https://github.com/hashicorp/terraform/tree/master/dag),
[init systems](https://www.freedesktop.org/wiki/Software/systemd/) and more.
walk(1) provides a generic primitive to express a DAG as a set of targets
(files) that depend on each other.

walk(1) can be used to build just about anything, from C/C++ programs to
frontend applications (css, sass, coffeescript, etc) and infrastructure
(CloudFormation, Terraform). Basically, anything that can express it's
dependencies can be built.

## OPTIONS

  * `-v`:
    Show stdout from the `Walkfile` when executing the **exec** phase.

  * `-j`=<number>:
    Controls the number of targets that are executed in parallel. By default,
    targets are executed with the maximum level of parallelism that the graph
    allows. To limit the number of targets that are executed in parallel, set
    this to a value greater than `1`. To execute targets serially, set this to
    `1`.

  * `-p`=<format>:
    Prints the underlying DAG to stdout, using the provided format. Available
    formats are `dot` and `plain`.

  * `--noprefix`:
    By default, the stdout/stderr output from the `Walkfile` is prefixed with the name
    of the target, followed by a tab character. This flag disables the
    prefixing. This can help with performance, or issues where you encounter
    "too many open files", since prefixing necessitates more file descriptors.

## TARGETS

Targets can be used to represent a task, or a file that needs to be built. They
are synonymous with targets in make(1). In general, targets are relative paths
to files that need to be built, like `src/hello.o`. When a target does not
relate to an actual file on disk, it's synonymous with `.PHONY` targets in
make(1).

walk(1) delegates to an executable file called [Walkfile][WALKFILE] within the same
directory as the target, to determine what dependencies the target has, and how
to execute it.

## WALKFILE

The `Walkfile` determines _how_ a target is executed, and what other targets it
depends on.

When walk(1) begins execution of a target, it attempts to find an executable
file called `Walkfile` in the same directory as the target, and then executes
it with the following positional arguments:

  * `$1`:
    The [phase][PHASES] (`deps` or `exec`).
  
  * `$2`:
    The name of the target to build (e.g. `hello.o`).

It's up to the `Walkfile` to determine what dependencies the target has, and
how to execute it.

It's generally a good practice to make the Walkfile return an error if it
doesn't know how to build a target, to prevent typos in targets.

    #!/bin/bash

    phase=$1
    target=$2

    case $target in
      *) >&2 echo "No rule for target \"$target\"" && exit 1 ;;
    esac

## PHASES

walk(1) has two phases:

  * `Plan`:
    In this phase, walk(1) executes the `Walkfile` with `deps` as the first
    argument. The `Walkfile` is expected to print a newline delimited list of
    files that the target depends on, relative to the target. Internally,
    walk(1) builds a graph of all of the targets and their dependencies.

  * `Exec`:
    In this phase, walk(1) executes the `Walkfile` with `exec` as the first
    argument. The `Walkfile` is expected to build the given target, but don't
    need to if it's, for example, a task (like `test`, `clean`, etc).

## COMPARISONS

walk(1) is heavily inspired by make(1) and
[redo](https://github.com/apenwarr/redo). There are a number of reasons why
walk(1) may be better in certain scenarios:

  * `Simplicity`:
    walk(1) does not have anything synonymous with make(1)'s
    [Makefile](https://www.gnu.org/software/make/manual/make.html). Everything
    is simply defined as executable files, which provides the ultimate level of
    flexibility on UNIX to compose existing tools.
  * `Conditional Execution`:
    For mostly legacy reasons, make(1) determines whether a target needs to be
    built based on the file modification time of its dependencies. While this
    works well for building C/C++ programs on a local machine, it breaks down
    in scenarios where you have a large complex build system that runs in a
    shared environment. Also, depending on the target, other caching
    mechanisms like content hashing may be more suitable. There are
    [attempts](http://blog.jgc.org/2006/04/rebuilding-when-hash-has-changed-not.html)
    to get around this handicap, but none that work well. walk(1) leaves
    conditional execution up to the Rule.
   * `Recursiveness`:
    Recursive make is generally a mistake. Whole papers have been [written
    about this topic](http://aegis.sourceforge.net/auug97.pdf). Because of
    walk(1)'s design, you can execute `walk` from any directory, and always get
    the same result. Recursiveness comes for free.

## EXAMPLES

For the following examples, we'll assume that we want to build a program called
[hello](https://github.com/ejholmes/walk/tree/master/test/111-compile) from
`hello.c` and `hello.h`. This can be expressed as a DAG, like the following:

                    all
                     |
                   hello
                     |
                  hello.o
                  /     \
              hello.c hello.h


When `walk` is invoked without any arguments, it defaults to a target called `all`:

    $ walk

Here's what happens within walk(1) when we execute this:

1. walk(1) resolves all of the dependencies, and builds a graph:

        $ Walkfile deps all
        hello
        $ Walkfile deps hello
        hello.o
        $ Walkfile deps hello.o
        hello.c
        hello.h
        $ Walkfile deps hello.c
        $ Walkfile deps hello.h

2. walk(1) executes all of the targets, starting with dependencies:

        $ Walkfile exec hello.c
        $ Walkfile exec hello.h
        $ Walkfile exec hello.o
        $ Walkfile exec hello
        $ Walkfile exec all

You can provide one or more targets as arguments to specify where to start
execution from. For example, if wanted to build just `hello.o` and any of it's
dependencies:

    $ walk hello.o

When targets are executed, they're always executed relative to the directory of
the target. This means that we can execute `walk` from any directory, and
always get the same behavior. All of the following are identical:

    $ walk hello.o
    $ cd .. && walk 111-compile/hello.o
    $ cd .. && walk test/111-compile/hello.o

See more at <https://github.com/ejholmes/walk/tree/master/test>.

## SIGNALS

When walk(1) receives SIGINT or SIGTERM, it will forward these signals down to
any targets that are currently executing. With that in mind, it's a good idea to
ensure that any potentially long running targets handle these signals to
terminate gracefully.

## BUGS

You can find a list of bugs at <https://github.com/ejholmes/walk/issues>.
Please report any issues there.

## COPYRIGHT

Walk is Copyright (C) 2017 Eric Holmes

## SEE ALSO

make(1), bash(1)

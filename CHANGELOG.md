# Changelog

## HEAD

**Improvements**

* Errors are now shown for failed/errored targets. [813434b](https://github.com/ejholmes/walk/commit/813434b77d23c4ce0209d7b1f0f9eecb0aaccf3b)

## 0.3.0 (2017-01-18)

**Features**

* walk(1) now simply delegates to a single `Walkfile` [#17](https://github.com/ejholmes/walk/pull/17)

## 0.2.1 (2017-01-17)

**Features**

* Lines are now prefixed with `ok` or `error`, and errored targets now appear in the output. [a9ad1a5](https://github.com/ejholmes/walk/commit/a9ad1a5c631dba7bc5707aa13df60e02e70a990b)
* Stdout/Stderr from rules is now prefixed with the target name. This can be disabled with the `--noprefix` flag. [9b19b53](https://github.com/ejholmes/walk/commit/9b19b537227d490aa8a658221ece81a8aae91a9b)

**Bugs**

* The error message shown when target(s) fail is not properly pluralized. [ff2fa28](https://github.com/ejholmes/walk/commit/ff2fa283af696285e29b4d6e742c52ea7be4d5f8)

## 0.2.0 (2017-01-04)

**Features**

* You can now print the underlying DAG in `dot` format using the `-p` flag. [c6104af](https://github.com/ejholmes/walk/commit/c6104afe20805929eb2a11d252c5b3b47a19acb5)
* You can now limit the number of targets that are built in parallel using the `-j` flag. [6ae6d0c](https://github.com/ejholmes/walk/commit/6ae6d0c231f00a76ff3871d782ab9bb57609b247)
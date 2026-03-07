# Changelog

## Unreleased

**Features**

* Walkfile inheritance: walk(1) now searches up the directory tree for a Walkfile, allowing a single Walkfile at the project root to handle targets in any subdirectory. This eliminates the need to create a Walkfile in every directory.

* Walkfile fallback (exit code 127): A local Walkfile can delegate to a parent Walkfile by exiting with code 127. This enables composition where a local Walkfile handles specific targets while inheriting generic rules from a parent:
  ```bash
  case $target in
    special) ;; # handle locally
    *) exit 127 ;; # delegate to parent
  esac
  ```

**Breaking Changes**

* **Target name (`$2`) format changed**: When a Walkfile is found in a parent directory, `$2` now contains the path relative to the Walkfile's directory (e.g., `subdir/foo.o` instead of `foo.o`). Walkfiles that assume `$2` is a simple filename may need adjustment:
  ```bash
  # Before: worked when $2 was "foo.o"
  echo ${target}.c

  # After: use basename or adjust patterns
  echo ${target%.o}.c  # works for both "foo.o" and "subdir/foo.o"
  ```

* **Working directory changed**: The Walkfile now runs from its own directory, not the target's directory. Commands using relative paths like `ls *.c` should be updated to use paths relative to the Walkfile.

* **Unintended Walkfile discovery**: Directories that previously had no Walkfile (treating targets as static files) may now inherit a Walkfile from a parent directory. If the parent Walkfile doesn't handle the target, this will produce an error instead of a no-op.

**Migration Guide**

1. If your Walkfile uses `$2` directly as a filename, consider using `basename "$target"` or updating patterns to handle paths.

2. If you rely on "no Walkfile means static file" behavior in a subdirectory, add an explicit case to the parent Walkfile:
   ```bash
   subdir/*) ;;  # treat as static files
   ```

3. Review any Walkfiles higher in your directory tree that might now be discovered unexpectedly.

## 0.3.3 (2017-09-20)

**Improvements**

* Dependencies specified as an absolute path are now handled properly. [ee548bf](https://github.com/ejholmes/walk/commit/ee548bf5f2cf7fc97fad77c710b27929b327833f)

## 0.3.2 (2017-08-18)

**Improvements**

* The `-d` flag has been removed, and a `plain` format has been added to `-p`. [383c9e5](https://github.com/ejholmes/walk/commit/383c9e5d296c8f372194a27e241a62e242c16b17)
* The help text has been expanded to match the man page more closely. [0292d61](https://github.com/ejholmes/walk/commit/0292d61ae8015149c190a5591f78d2abea22d9e6)

## 0.3.1 (2017-01-23)

**Improvements**

* Errors are now shown for failed/errored targets. [813434b](https://github.com/ejholmes/walk/commit/813434b77d23c4ce0209d7b1f0f9eecb0aaccf3b)
* If a directory doesn't contain a Walkfile, walk(1) will no longer attempt to execute it. [0502d1f](https://github.com/ejholmes/walk/commit/0502d1f8eab49d1b0724dacc068fb812729bc75c)

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

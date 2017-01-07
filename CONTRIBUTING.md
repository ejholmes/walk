# Contributing Guidelines

## Development

walk(1) is written in Go, and it's assumed that you have a working Go 1.7+ environment.

## Tests

There are multiple test suites that are exercised. The full suite requires that walk is installed:

```console
$ go install .
$ walk clean && walk test
```

The Go specific tests can be ran with:

```console
go test .
```

# Contributing Guidelines

## Development

walk(1) is written in Go, and requires Go 1.23+.

## Tests

There are multiple test suites that are exercised. The full suite requires that walk is installed:

```console
$ go install .
$ walk clean && walk test/unit && walk test/integration
```

The Go specific tests can be ran with:

```console
go test ./...
```

## Releasing

Bump the version number in [VERSION](./VERSION), update CHANGELOG.md, then run:

```console
$ walk -v version.go
$ git commit -m "Bump version"
$ walk -v release
```

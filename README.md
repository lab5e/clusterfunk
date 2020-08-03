# Clusterfunk

[GoDoc documentation @ pkg.go.dev](https://pkg.go.dev/github.com/lab5e/clusterfunk)

## What it is

This is a cluster library for Go. At its core it uses Raft and Serf to manage
and automate clusters of nodes.

## What it isn't

A silver bullet.

## Requirements

clusterfunk requires Go version 1.12 or later. You'll need a few linters to
run the lint target but you can build a version with `make nolint` which will
just run the tests and build the binaries.

## Building

This library requires a fairly recent version of Go, at least 1.12. If you are
modifying the protobuffer files you'll need the protoc compiler. The generated
files are included in the source tree so you won't need it unless you use
`make generate` (or `go generate ./...`).

Run `make` to build the demo service. The build uses a host of checkers (golint,
go vet, staticcheck, revive and golangci-lint) so you might get some errors if
you miss one or more of those.

Get them the usual way:

```text
go get -u golang.org/x/lint/golint
go get honnef.co/go/tools/cmd/staticcheck
go get -u github.com/mgechev/revive
# This only works for macOS, check out https://github.com/golangci/golangci-lint#install for other platforms
# go get works as well but the author doesn't recommend it.
brew install golangci/tap/golangci-lint
```

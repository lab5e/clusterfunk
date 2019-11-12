# Clusterfunk

## Building

This library requires a fairly recent version of Go, at least 1.12. If you are modifying the protobuffer files you'll need the protoc compiler. The generated files are included in the source tree so you won't need it unless you use `make generate` (or `go generate ./...`).

Run `make` to build the demo service. The build uses a host of checkers (golint, go vet, staticcheck, revive and golangci-lint) so you might get some errors if you miss one or more of those.

Get them the usual way:
```text
go get -u golang.org/x/lint/golint
go get honnef.co/go/tools/cmd/staticcheck
go get -u github.com/mgechev/revive
# This only works for macOS, check out https://github.com/golangci/golangci-lint#install for other platforms
# go get works as well but the author doesn't recommend it.
brew install golangci/tap/golangci-lint
```

## What it is

This is a cluster library for Go. At its core it uses Raft and Serf to manage
and automate clusters of nodes.

## What it isn't

A silver bullet.

## Requirements

clusterfunk requires Go version 1.12 or later.

## Setting up a cluster

## Managing the cluster

## Sharding

### Shard functions

### Shard weights

## Limitations

## Replicated log

The replicated log is kept quite simple. The first byte indicates the message type
and any log message type with an higher index overwrites the previous message in
the log.

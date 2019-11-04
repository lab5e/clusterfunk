# Clusterfunk

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

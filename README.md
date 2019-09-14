# Clusterfunk

## What it is

This is a cluster library for Go. At its core it uses Raft and Serf to manage
and automate clusters of nodes.

## What it isn't

A silver bullet.

## Setting up a cluster

## Managing the cluster

## Sharding

### Shard functions

### Shard weights

## Limitations

## TODOs

* mutex when accessing the Raft instance (panics are not great when shutting down)
* Proper FSM and log
* Replicate logs with SQLite
* Turn off auto-join/leave for Serf in production clusters
* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
having one or more nodes registered in DNS makes sense. Might need
some additional machinery on the outside.
* Make utility functions for common operations (discover zeroconf, create
serf client etc)
* Open source the parameters library (flags is cumbersome)
* Complete demo with work spread across nodes
* gRPC for leader tasks (redistribute shards, node join et al)


### Note on timing

All timings are on an i7 processor @ 3 GHz. YMMV. Typical size for the inital
clusters are determined by how many Raft can handle. Anything above 9 nodes
yields nervous ticks for the authors but there are ways around this.
Test data set is 10k shards on 50 nodes. Everything runs on the same host for
cluster tests; ie the RTT for the network is practically 0. For AWS we can expect
RTT in the range of 1-2ms.

| What | Time | Comments |
| - | - | - |
| Detecting a failed node | - | This can be tuned by adjusting heart beats. Higher rate means more false positives if a node is under load |
| Redistributing shards | .2ms | This is just the processing time |
| Encoding shards for the log | 2ms | This can be sped up by encoding manually |
| Decoding shards from the log | 3ms | This can also be sped up by manual decoding into a byte buffer |
| Replicating logs | .5 - 1.5ms | Measured by using ApplyLog |
| Shard -> Node lookup | 80ns | This is a simple array lookup |
| Initialize new shard manager | .45ms | Used when leader starts up |
| Computing shard from integer key | 27ns | |
| Computing shard from string key | 108ns | |


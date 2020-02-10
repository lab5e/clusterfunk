# Clusterfunk

`clusterfunk` is a Go library that enables you to create clusters of services. The library assists you in setting up, configuration and spreading the load across the nodes in the cluster.

## Building

Use `make` to build the demo server and client.

## Features

* Zeroconf cluster discovery
* In-memory log for development and testing
* Simple endpoint discovery via Serf
* Server-side proxying of requests to other cluster nodes
* Client-side connection pooling
* Metrics to measure cluster health
* Quick failover (typically less than 1 second when a node fails)

## Limitations

A typical cluster of Raft nodes should not exceed 10-15 nodes. This is a limitation in the Raft protocol (and library).

## What it isn't

* A silver bullet. You can't (at least not yet) throw it in front of a random service and tick the "redundant cluster" check box on your requirement list :) You have to spend a while thinking how you want the requests distributed across your nodes (that's the sharding bit) and how you are going to handle errors, what to shard and how to shard.
* A load balancer. If you just need a load balancer in front of your service do use one. It's much easier to install, manage and use.
* A performance enhancing library. If your service is slow it won't make it go faster. You might get better throughput if you do all the required steps (see [Little's Law](https://en.wikipedia.org/wiki/Little%27s_law)) but if your backend storage is a limiting factor it might end up being slower than before since it introduces network hops.

* [Introduction](introduction.md)
* [CtrlC tool](ctrlc.md)
* [Demo service](demo.md)
* [Implementing a server](servers.md)
* [Implementing a client](clients.md)
* [Sharding](sharding.md)
* [Production services](production.md)
* [Notes on networking](networking.md)
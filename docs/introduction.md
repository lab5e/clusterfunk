![Clusterfunk](img/cf_logo_280x50.png)

# Introduction

`clusterfunk` is a Go library that enables you to create clusters of services. The library assists you in setting up, configuration and spreading the load across the nodes in the cluster. 

## Features

* Zeroconf cluster discovery
* Simple endpoint discovery via Serf
* Server-side proxying of requests to other cluster nodes
* Client-side connection pooling
* Metrics to measure cluster health
* Quick failover (typically less than 1 second when a node fails)

## Limitations

A typical cluster of Raft nodes should not exceed 10-15 nodes. This is a limitation in the Raft protocol (and library).

## What it is

## What it isn't

* A silver bullet. You can't (at least not yet) throw it in front of a random service and tick the "redundant cluster" check box on your requirement list :) You have to spend a while thinking how you want the requests distributed across your nodes (that's the sharding bit) and how you are going to handle errors, what to shard and how to shard.
* A load balancer. If you just need a load balancer in front of your service do use one. It's much easier to install, manage and use.
* A performance enhancing library. If your service is slow it won't make it go faster. You might get better throughput if you do all the required steps (see [Little's Law](https://en.wikipedia.org/wiki/Little%27s_law)) but if your backend storage is a limiting factor it might end up being slower than before since it introduces network hops.

## Principles

A **cluster** contains one or more **nodes**. A node can be a process, a virtual machine, a container or a separate computer. The nodes in the cluster selects one of the nodes as the **leader** of the cluster. If the leader steps down or becomes unavailable the remaining nodes elect a new leader. The nodes in the cluster split the work between them and the leader is responsible for dividing the load between the nodes. The leader processes requests in the same way as the rest of the nodes.

The work is divided through **shards**. All requests map to one and only one shard. Each node handles several shards in the cluster and is responsible for all requests that map to the shards it owns. The number of shards is much larger than the number of nodes in the cluster. If a node gets a request that maps to a shard it doesn't own it will proxy the request to the correct node.

The shards are represented as simple integers and the shard mapping function can be expressed as `f(request) = n` where `n` is the shard. The shard map is an array of assignments to the nodes in the cluster. The index of the array is the shard identifier. A (very simple) shard map for the nodes A, B, C with nine shards looks like the following:

```
shardMap := [ "A", "B", "C", "A", "B", "C", "A", "B", "C"]
```

* Node A is responsible for shard 0, 3 and 6
* Node B is responsible for shard 1, 4 and 7
* Node C is responsible for shard 2, 5 and 8

## Rules

* A node can't serve requests unless a leader is elected.
* A node can only serve requests that maps to one of the shards the node owns.
* The leader will generate the shard map and write it to the replicated log.
* A node can't serve requests without a commited shard map.
* The shard map is distributed via the cluster's replicated log
* Requests will be queued while there's a leader election in progress or a resharding process has started.
* The shard map must be confirmed by all nodes before it is committed
* If a node goes away the leader starts resharding the cluster.

## Sharding the cluster

When a node leaves the cluster or a leader is elected the nodes stop serving requests. The leader generates a new shard map and distributes it via the replicated log. The shard map is acknowledged by the nodes through a separate channel. If one of the nodes fails to acknowledge the shard map a new map is generated without the node and the process is repeated. Once the shard map is confirmed by the nodes the committed shard map is distributed via the replicated log.

Nodes acknowledge the shard map to the leader itself and the leader replies with OK if this is the current shard map or ERROR and the log entry for the current shard map index.

When the nodes receive the confirmed shard map entry they start serving requests again.

The clock on each node is not relevant since the replicated log is used as a timing mechanism for the nodes. If a node is lagging behind in the replicated logs it won't get an acknowledgement from the current leader. A node without a known leader can't serve requests.


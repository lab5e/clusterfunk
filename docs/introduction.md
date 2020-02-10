# General

A **cluster** contains one or more **nodes**. A node can be a process, a virtual machine, a container or a separate computer. The nodes in the cluster selects one of the nodes as the **leader** of the cluster. If the leader steps down or becomes unavailable the remaining nodes elect a new leader. The nodes in the cluster split the work between them and the leader is responsible for dividing the load between the nodes. The leader processes requests in the same way as the rest of the nodes.

The work is divided through **shards**. All requests map to one and only one shard. Each node handles several shards in the cluster and is responsible for all requests that map to the shards it owns. The number of shards is much larger than the number of nodes in the cluster. If a node gets a request that maps to a shard it doesn't own it will proxy the request to the correct node.

Each node exposes one or more endpoints. The endpoints are announced via the Serf cluster.

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


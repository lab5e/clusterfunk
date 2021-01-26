# Restoring clusters that have died or can't establish a quorum

This is the slightly unpopular and impossible bit - what happens if a majority
of nodes have died and the remaining nodes are left in a state where they can't
recover. Since there's no leader you can't remove members from the cluster and
you can't get a leader unless you add more nodes.

*Technically* this should not happen but - yeah. It might do.

At this point you have two options:

1. Create a new cluster by bootstrapping a new node and then recreate the nodes.
   This *might* be the quickest solution if you don't care about the replicated
   log contents. Wipe all the log data from disk if they're persisted, bootstrap
   the new cluster and be done with it.
2. Recover the nodes. If you are able to move IP addresses around and get
   another node up with the same endpoint and node ID the cluster might be able
   to recover into a working state. The Raft endpoints in use can be

In a production setting the raft nodes should be stable and repeatable, ie the
Raft endpoint, Node ID and memberships should be static and not auto generated, ie
the following parameters must be set on each node:

* `cluster-auto-join=false` to disable automatic joins via Serf events. This is
  convinient when testing and firing up ad hoc clusters but could be bad if the
  Serf nodes manage to communicate in a split brain scenario.
* `cluster-node-id=[fixed node id]` to set the Node ID for each cluster node.
* `cluster-raft-endpoint=[fixed endpoint for Raft]` to set the Raft endpoint the
  nodes use. This is the externally visible IP address that the Raft nodes use
  to identify each node.
* `cluster-raft-bootstrap=false` to prevent automatic bootstrapping of new clusters
  when a node is launched.

This *does* require extra steps when setting up a cluster but nodes that restarts
won't be removed by the leader. If a node dies the leader will reshard the cluster
and omit the non-responsive node from the shard mapping. Once the node comes
back up it will get its fair share of shards again.

## Bootstrapping a new cluster

This uses the demo server and logs are persisted in `./logs`, one directory for
each node. All nodes runs on the same computer and its public IP is 192.168.1.99

Bootstrap a new node:

```bash
bin/demo \
    --cluster-node-id=node1 \
    --cluster-raft-endpoint=192.168.1.99:50001 \
    --cluster-raft-bootstrap=true \
    --cluster-auto-join=false
```

Launch a new node. This node won't join the cluster automatically and just
wait:

```bash
bin/demo \
    --cluster-node-id=node2 \
    --cluster-raft-disk-store=logs \
    --cluster-raft-bootstrap=false \
    --cluster-raft-endpoint=192.168.1.99:50002 \
    --cluster-auto-join=false
```

Add the node via the `ctrlc` tool:

```bash
bin/ctrlc node add --id=node2
```

The cluster should now contain two nodes

Launch the third node:

```bash
bin/demo \
    --cluster-node-id=node3 \
    --cluster-raft-disk-store=logs \
    --cluster-raft-bootstrap=false \
    --cluster-raft-endpoint=192.168.1.99:50003
```

Finally, add the third node via `ctrlc`:

```bash
bin/ctrlc node add --id=node3
```

The cluster now has three nodes. Stop the first node and omit the bootstrap flag
to get to a proper runtime configuration for all three nodes:

# ctrlc - cluster control tool

This tool is used to manage the cluster nodes via gRPC.

## Usage

```ctrlc [args] [command] [command args]```

...where `args` is zero or more of:

* `--cluster-name [string]` - name of cluster. Default is `demo`
* `--zeroconf [bool]`  -- use zeroconf to locate nodes. Default is `true`
* `--endpoint [string]` -- management endpoint to use. This will not use zeroconf and ignore the cluster name parameter.

...and `command` is one of:

* `status` -- show status of cluster.
* `nodes` -- list nodes in cluster. The cluster must be operational.
* `endpoints [name]` -- list endpoints. If name is omitted all known endpoints will be listed.
* `add-node [id]` - add a Raft node to the cluster. Node ID is required and the node must have joined the Serf cluster. The node will be added immediately and the cluster will start resharding. The cluster must be operational.
* `remove-node [id]` -- remove a Raft node from the cluster. Node ID is required. The node will be drained and removed immediately and the cluster will start resharding. The cluster must be operational.
* `shards` -- list shard distribution per node in the cluster. The cluster must be operational.
* `step-down` -- force a re-election in the cluster. The cluster must be operational.

The default is to use Zeroconf to locate the first management endpoint. Note that the node might be unavailable.

Care should be taken when using this tool. It might make the cluster unavailable (particularly `remove-node` and `step-down`)

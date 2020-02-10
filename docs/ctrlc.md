
# ctrlc - the control cluster command

This is a command to manage the cluster. It can be used as is or integrated your own tools. It uses the Kong library for commands and the command structure in `pkg/ctrlc` can be merged with an existing structure. If you use a different library for command line handling the various commands can be invoked directly with the parameter struct as an argument. They will print directly to stdout.

The tool can connect to any node in the cluster. If a command must be executed
on the leader the request will be proxied to the leader itself.

## Configuration parameters

* `--help` - show help for commands
* `-n, --cluster-name="clusterfunk"` - set the cluster name. This parameter is ignored if you use the endpoint parameter to connect directly to a node.
* `-z, --zeroconf`  - use zeroconf for discovery. The name parameter is used to separate between different clusters running on the local network.
* `-e, --endpoint=STRING` - a host:port string that you can use to connect directly to a node in the cluster rather than using ZeroConf/mDNS for discovery.
* `-T, --tls` - enable TLS when communicating with the cluster nodes.
* `-C, --cert-file=STRING` - client certificate for the service.
* `-H, --hostname-override=STRING` - host name for the TLS certificate. If the name resolves to the same name as in the client certificate this is optional.

## Commands

### `status` command

Show the node's current status.

### `nodes` command

List the nodes in the cluster.

### `endpoints` command

Query the endpoints in the cluster. You can filter by a (plain) string, ie. if
you want to see all management endpoints you can use "ep.management", "management"
or just "mana".

### `node add` and `node rm` commands

Add and remove nodes to the cluster. This will (obviously) require a cluster
resharding when a node is added or removed.

### `shards` command

List the shard distribution in the cluster.

### `step-down` command

This will make the current leader in the cluster step down and a new leader is
elected. This also forces a cluster resharding.

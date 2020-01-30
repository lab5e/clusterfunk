# ctrlc - cluster control tool

This tool is used to manage the cluster nodes via gRPC.

## Usage

```text
Usage: ctrlc <command>

Clusterfunk management CLI

Flags:
      --help                          Show context-sensitive help.
  -n, --cluster-name="clusterfunk"    Cluster name
  -z, --zeroconf                      Use zeroconf discovery for Serf
  -e, --endpoint=STRING               gRPC management endpoint
  -T, --tls                           TLS enabled for gRPC
  -C, --cert-file=STRING              Client certificate for management service
  -H, --hostname-override=STRING      Host name override for certificate

Commands:
  status       Show the node status
  nodes        List the nodes in the cluster
  endpoints    List endpoints known by the node
  node add     Add node to cluster
  node rm      Remove node from cluster
  shards       Show the shards in the cluster
  step-down    Step down as the current leader

Run "ctrlc <command> --help" for more information on a command.
```

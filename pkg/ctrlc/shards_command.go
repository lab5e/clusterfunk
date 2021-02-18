package ctrlc

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// ShardsCommand is the subcommand that shows the shards in the cluster
type ShardsCommand struct {
}

// Run shows the current shard distribution in the cluster
func (c *ShardsCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClientParams())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()

	res, err := client.ListShards(ctx, &managepb.ListShardsRequest{})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing shards: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Unable to list shards: %v\n", res.Error.Message)
		return errStd
	}

	table := tabwriter.NewWriter(os.Stdout, 1, 3, 1, ' ', 0)
	table.Write([]byte("Node ID\tShards\tPercent\n"))
	for _, v := range res.Shards {
		shardPct := float32(v.ShardCount) / float32(res.TotalShards) * 100.0
		table.Write([]byte(fmt.Sprintf("%s\n%d\t(%3.1f%%)\n", v.NodeId, v.ShardCount, shardPct)))
	}
	table.Flush()
	fmt.Printf("\nReporting node: %s    Total shards: %d\n", res.NodeId, res.TotalShards)

	return nil
}

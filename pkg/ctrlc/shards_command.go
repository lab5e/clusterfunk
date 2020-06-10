package ctrlc

import (
	"context"
	"fmt"
	"os"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/managepb"
)

// ShardsCommand is the subcommand that shows the shards in the cluster
type ShardsCommand struct {
}

// Run shows the current shard distribution in the cluster
func (c *ShardsCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
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

	fmt.Println("Node ID              Shards             Weight")
	for _, v := range res.Shards {
		shardPct := float32(v.ShardCount) / float32(res.TotalShards) * 100.0
		weightPct := float32(v.ShardWeight) / float32(res.TotalWeight) * 100.0
		fmt.Printf("%-20s %10d (%3.1f%%) %10d (%3.1f%%)\n", v.NodeId, v.ShardCount, shardPct, v.ShardWeight, weightPct)
	}
	fmt.Printf("\nReporting node: %s    Total shards: %d    Total weight: %d\n", res.NodeId, res.TotalShards, res.TotalWeight)

	return nil
}

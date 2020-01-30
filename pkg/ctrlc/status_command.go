package ctrlc

import (
	"context"
	"fmt"
	"os"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/clustermgmt"
)

// StatusCommand is a subcommand for the ctrlc CLI.
type StatusCommand struct {
}

// Run executes the status operation
func (c *StatusCommand) Run(args RunContext) error {
	client := connectToManagement(args.ServerParameters())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.GetStatus(ctx, &clustermgmt.GetStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving status: %v\n", err)
		return errStd
	}
	fmt.Printf("Cluster name: %s\n", res.ClusterName)
	fmt.Printf("Node ID:      %s\n", res.LocalNodeId)
	fmt.Printf("State:        %s\n", res.LocalState)
	fmt.Printf("Role:         %s\n", res.LocalRole)
	fmt.Printf("Leader ID:    %s\n", res.LeaderNodeId)
	fmt.Printf("Nodes:        %d Raft, %d Serf\n", res.RaftNodeCount, res.SerfNodeCount)
	fmt.Printf("Shards:       %d (total weight: %d)\n", res.ShardCount, res.ShardWeight)

	return nil
}

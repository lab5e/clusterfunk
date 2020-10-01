package ctrlc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// StatusCommand is a subcommand for the ctrlc CLI.
type StatusCommand struct {
}

// Run executes the status operation
func (c *StatusCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.GetStatus(ctx, &managepb.GetStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving status: %v\n", err)
		return errStd
	}
	t := time.Unix(0, res.Created)
	d := time.Since(t)
	fmt.Printf("Cluster name: %s\n", res.ClusterName)
	fmt.Printf("Created at    %s  (%dd, %02dh, %02dm, %02ds ago)\n", t.Format(time.RFC822),
		int64(d.Hours()/24), int64(d.Hours()), int64(d.Minutes()), int64(d.Seconds()))
	fmt.Printf("Node ID:      %s\n", res.LocalNodeId)
	fmt.Printf("State:        %s\n", res.LocalState)
	fmt.Printf("Role:         %s\n", res.LocalRole)
	fmt.Printf("Leader ID:    %s\n", res.LeaderNodeId)
	fmt.Printf("Nodes:        %d Raft, %d Serf\n", res.RaftNodeCount, res.SerfNodeCount)
	fmt.Printf("Shards:       %d (total weight: %d)\n", res.ShardCount, res.ShardWeight)

	return nil
}

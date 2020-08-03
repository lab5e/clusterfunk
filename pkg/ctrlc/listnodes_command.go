package ctrlc

import (
	"context"
	"fmt"
	"os"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// ListNodesCommand is the subcommand to list nodes in the cluster
type ListNodesCommand struct {
}

// Run executes the list nodes command
func (c *ListNodesCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()

	res, err := client.ListNodes(ctx, &managepb.ListNodesRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing nodes: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Unable to list nodes: %v\n", res.Error.Message)
		return errStd
	}

	fmt.Printf("  Node ID              Raft       Serf\n")
	for _, v := range res.Nodes {
		leader := ""
		if v.Leader {
			leader += "*"
		}
		fmt.Printf("%-2s%-20s %-10s %s\n", leader, v.NodeId, v.RaftState, v.SerfState)
	}
	fmt.Printf("\nReporting node: %s   Leader node: %s\n", res.NodeId, res.LeaderId)
	return nil
}

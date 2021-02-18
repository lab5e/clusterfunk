package ctrlc

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// ListNodesCommand is the subcommand to list nodes in the cluster
type ListNodesCommand struct {
	ShowAll bool `kong:"help='Show all (ie incl failed, dead and left nodes)'"`
}

// Run executes the list nodes command
func (c *ListNodesCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClientParams())
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

	table := tabwriter.NewWriter(os.Stdout, 1, 3, 1, ' ', 0)
	table.Write([]byte("\tNode ID\tRaft\tSerf\n"))
	for _, v := range res.Nodes {
		leader := ""
		if v.Leader {
			leader += "*"
		}
		table.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\n", leader, v.NodeId, v.RaftState, v.SerfState)))
	}
	table.Flush()
	fmt.Printf("\nReporting node: %s   Leader node: %s\n", res.NodeId, res.LeaderId)
	return nil
}

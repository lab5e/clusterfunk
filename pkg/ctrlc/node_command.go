package ctrlc

import (
	"context"
	"fmt"
	"os"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/managepb"
)

// NodeCommand is the subcommand to add and remove nodes
type NodeCommand struct {
	Add addNodeCommand    `kong:"cmd,help='Add node to cluster'"`
	Rm  removeNodeCommand `kong:"cmd,help='Remove node from cluster'"`
	ID  string            `kong:"required,help='Node ID',short='N'"`
}

type addNodeCommand struct {
}

func (c *addNodeCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
	if client == nil {
		return errStd
	}
	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.AddNode(ctx, &managepb.AddNodeRequest{
		NodeId: args.ClusterCommands().Node.ID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding node: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Leader could not add node: %v\n", res.Error.Message)
		return errStd
	}
	fmt.Printf("Node %s added to cluster\n", args.ClusterCommands().Node.ID)
	return nil
}

type removeNodeCommand struct {
}

func (c *removeNodeCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.RemoveNode(ctx, &managepb.RemoveNodeRequest{
		NodeId: args.ClusterCommands().Node.ID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing node: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Leader could not remove node: %v\n", res.Error.Message)
		return errStd
	}
	fmt.Printf("Node %s removed from cluster\n", args.ClusterCommands().Node.ID)

	return nil
}

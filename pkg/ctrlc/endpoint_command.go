package ctrlc

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// EndpointsCommand is the subcommand to list endpoints
type EndpointsCommand struct {
	Filter string `kong:"optional,arg,help='Filter on prefix'"`
}

// Run executes the endpoint command
func (c *EndpointsCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClusterServer())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()

	res, err := client.FindEndpoint(ctx, &managepb.EndpointRequest{EndpointName: args.ClusterCommands().Endpoints.Filter})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching for endpoint: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Unable to search for endpoint: %v\n", res.Error.Message)
		return errStd
	}

	fmt.Printf("Node ID              Name                 Endpoint\n")

	sort.Slice(res.Endpoints, func(i, j int) bool {
		return res.Endpoints[i].Name < res.Endpoints[j].Name
	})
	for _, v := range res.Endpoints {
		fmt.Printf("%-20s %-20s %s\n", v.NodeId, v.Name, v.HostPort)
	}
	fmt.Printf("\nReporting node: %s\n", res.NodeId)

	return nil
}

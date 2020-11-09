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

	services := make(map[string]string)
	// Sort endpoints by node, then print list of endpoints under each node
	endpoints := make(map[string][]*managepb.EndpointInfo)
	for _, v := range res.Endpoints {
		services[v.NodeId] = v.ServiceName
		epList, ok := endpoints[v.NodeId]
		if !ok {
			epList = make([]*managepb.EndpointInfo, 0)
		}
		epList = append(epList, v)
		endpoints[v.NodeId] = epList
	}
	nodes := make([]string, 0)
	for k := range endpoints {
		nodes = append(nodes, k)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})

	fmt.Printf("Endpoints for cluster '%s'\n", args.ClusterServer().Name)
	fmt.Printf("------------------------------------------------\n")
	for _, nodeid := range nodes {
		fmt.Printf("Node: %s, Service: %s\n", nodeid, services[nodeid])
		epList := endpoints[nodeid]
		sort.Slice(epList, func(i, j int) bool {
			return epList[i].Name < epList[j].Name
		})
		for i, ep := range epList {
			ch := '|'
			if i == (len(epList) - 1) {
				ch = '\\'
			}
			fmt.Printf("  %c- %s -> %s\n", ch, ep.Name, ep.HostPort)
		}
		fmt.Println()
	}

	fmt.Printf("\nReporting node: %s\n", res.NodeId)

	return nil
}

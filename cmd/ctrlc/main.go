package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/stalehd/clusterfunk/clientfunk"
	"github.com/stalehd/clusterfunk/funk/clustermgmt"
	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

type parameters struct {
	ClusterName string `param:"desc=Cluster name;default=demo"`
	Zeroconf    bool   `param:"desc=Use zeroconf discovery for Serf;default=true"`
	Endpoint    string `param:"desc=Management endpoint to use"`
}

const (
	cmdStatus     = "status"
	cmdNodes      = "nodes"
	cmdEndpoints  = "endpoints"
	cmdAddNode    = "add-node"
	cmdRemoveNode = "remove-node"
	cmdShards     = "shards"
	cmdStepDown   = "step-down"
)

type commandRunner func(clustermgmt.ClusterManagementClient, parameters)

func main() {
	var config parameters
	flag.StringVar(&config.ClusterName, "cluster-name", "demo", "Name of cluster to connect with")
	flag.BoolVar(&config.Zeroconf, "zeroconf", true, "Use Zeroconf to locate endpoints")
	flag.StringVar(&config.Endpoint, "endpoint", "", "Management endpoint")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command specfied\n")
		return
	}

	if config.Endpoint == "" && config.Zeroconf {
		if config.Zeroconf && config.ClusterName == "" {
			fmt.Fprintf(os.Stderr, "Needs a cluster name if zeroconf is to be used for discovery")
			return
		}
		ep, err := clientfunk.ZeroconfManagementLookup(config.ClusterName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to do zeroconf lookup: %v\n", err)
			return
		}
		config.Endpoint = ep
	}

	if config.Endpoint == "" {
		fmt.Fprintf(os.Stderr, "Need an endpoint for one of the cluster nodes")
		return
	}

	grpcParams := toolbox.GRPCClientParam{
		ServerEndpoint: config.Endpoint,
	}
	opts, err := toolbox.GetGRPCDialOpts(grpcParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create GRPC dial options: %v\n", err)
		return
	}
	conn, err := grpc.Dial(config.Endpoint, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not dial management endpoint: %v\n", err)
		return
	}
	client := clustermgmt.NewClusterManagementClient(conn)

	switch args[0] {
	case cmdStatus:
		status(client)

	case cmdAddNode:
		if len(args) != 2 {
			fmt.Fprintf(os.Stderr, "Need ID for %s\n", cmdAddNode)
			return
		}
		addNode(args[1], client)

	case cmdRemoveNode:
		if len(args) != 2 {
			fmt.Fprintf(os.Stderr, "Need ID for %s\n", cmdRemoveNode)
			return
		}
		removeNode(args[1], client)

	case cmdEndpoints:
		epFilter := ""
		if len(args) > 1 {
			epFilter = args[1]
		}
		listEndpoints(epFilter, client)

	case cmdNodes:
		listNodes(client)

	case cmdShards:
		listShards(client)

	case cmdStepDown:
		stepDown(client)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		return
	}
}

const (
	gRPCTimeout = 1 * time.Second
)

func status(client clustermgmt.ClusterManagementClient) {
	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.GetStatus(ctx, &clustermgmt.GetStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving status: %v\n", err)
		return
	}
	fmt.Printf("Cluster name: %s\n", res.ClusterName)
	fmt.Printf("Node ID:      %s\n", res.LocalNodeId)
	fmt.Printf("State:        %s\n", res.LocalState)
	fmt.Printf("Role:         %s\n", res.LocalRole)
	fmt.Printf("Leader ID:    %s\n", res.LeaderNodeId)
	fmt.Printf("Nodes:        %d Raft, %d Serf\n", res.RaftNodeCount, res.SerfNodeCount)
	fmt.Printf("Shards:       %d (total weight: %d)\n\n", res.ShardCount, res.ShardWeight)
}

func addNode(id string, client clustermgmt.ClusterManagementClient) {
	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.AddNode(ctx, &clustermgmt.AddNodeRequest{
		NodeId: id,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding node: %v\n", err)
		return
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Leader could not add node: %v\n", res.Error.Message)
		return
	}
	fmt.Printf("Node %s added to cluster\n", id)
}

func removeNode(id string, client clustermgmt.ClusterManagementClient) {
	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.RemoveNode(ctx, &clustermgmt.RemoveNodeRequest{
		NodeId: id,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing node: %v\n", err)
		return
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Leader could not remove node: %v\n", res.Error.Message)
		return
	}
	fmt.Printf("Node %s removed from cluster\n", id)
}

func listEndpoints(filter string, client clustermgmt.ClusterManagementClient) {
	fmt.Fprintf(os.Stderr, "list endpoints")
}

func listNodes(client clustermgmt.ClusterManagementClient) {
	fmt.Fprintf(os.Stderr, "list nodes")
}

func listShards(client clustermgmt.ClusterManagementClient) {
	fmt.Fprintf(os.Stderr, "list shards")
}

func stepDown(client clustermgmt.ClusterManagementClient) {
	fmt.Fprintf(os.Stderr, "step down")
}

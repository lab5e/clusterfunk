package main

import (
	"flag"
	"fmt"

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
		fmt.Println("No command specfied")
		return
	}

	if config.Endpoint == "" && config.Zeroconf {
		if config.Zeroconf && config.ClusterName == "" {
			fmt.Printf("Needs a cluster name if zeroconf is to be used for discovery")
			return
		}
		ep, err := clientfunk.ZeroconfManagementLookup(config.ClusterName)
		if err != nil {
			fmt.Printf("Unable to do zeroconf lookup: %v\n", err)
			return
		}
		config.Endpoint = ep
	}

	if config.Endpoint == "" {
		fmt.Println("Need an endpoint for one of the cluster nodes")
		return
	}

	grpcParams := toolbox.GRPCClientParam{
		ServerEndpoint: config.Endpoint,
	}
	opts, err := toolbox.GetGRPCDialOpts(grpcParams)
	if err != nil {
		fmt.Printf("Could not create GRPC dial options: %v\n", err)
		return
	}
	conn, err := grpc.Dial(config.Endpoint, opts...)
	if err != nil {
		fmt.Printf("Could not dial management endpoint: %v\n", err)
		return
	}
	client := clustermgmt.NewClusterManagementClient(conn)

	switch args[0] {
	case cmdAddNode:
		if len(args) != 2 {
			fmt.Printf("Need ID for %s\n", cmdAddNode)
			return
		}
		addNode(args[1], client)

	case cmdRemoveNode:
		if len(args) != 2 {
			fmt.Printf("Need ID for %s\n", cmdRemoveNode)
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
		fmt.Printf("Unknown command: %s\n", args[0])
		return
	}
}

func addNode(id string, client clustermgmt.ClusterManagementClient) {
	fmt.Println("add node")
}

func removeNode(id string, client clustermgmt.ClusterManagementClient) {
	fmt.Println("remove node")
}

func listEndpoints(filter string, client clustermgmt.ClusterManagementClient) {
	fmt.Println("list endpoints")
}

func listNodes(client clustermgmt.ClusterManagementClient) {
	fmt.Println("list nodes")
}

func listShards(client clustermgmt.ClusterManagementClient) {
	fmt.Println("list shards")
}

func stepDown(client clustermgmt.ClusterManagementClient) {
	fmt.Println("step down")
}

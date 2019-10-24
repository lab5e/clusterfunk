package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/stalehd/clusterfunk/clientfunk"
	"github.com/stalehd/clusterfunk/funk/clustermgmt"
	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

type parameters struct {
	ClusterName  string `param:"desc=Cluster name;default=demo"`
	Zeroconf     bool   `param:"desc=Use zeroconf discovery for Serf;default=true"`
	Command      string `param:"desc=Command to execute;option=get-state,list-ndoes"`
	ShowInactive bool   `param:"desc=Show inactive nodes;default=false"`
	GRPCClient   toolbox.GRPCClientParam
}

const (
	cmdGetState  = "get-state"
	cmdListNodes = "list-nodes"
)

type commandRunner func(clustermgmt.ClusterManagementClient, parameters)

func main() {
	var config parameters
	flag.StringVar(&config.ClusterName, "cluster-name", "demo", "Name of cluster to connect with")
	flag.BoolVar(&config.Zeroconf, "zeroconf", true, "Use Zeroconf to locate endpoints")
	flag.StringVar(&config.GRPCClient.ServerEndpoint, "endpoint", "", "Management server endpoint")
	flag.StringVar(&config.Command, "cmd", "get-state", "Command to execute (get-state, list-nodes)")
	flag.BoolVar(&config.ShowInactive, "show-inactive", false, "Show inactive nodes in lists")
	flag.Parse()

	if config.GRPCClient.ServerEndpoint == "" && config.Zeroconf {
		if config.Zeroconf && config.ClusterName == "" {
			fmt.Printf("Needs a cluster name if zeroconf is to be used for discovery")
			return
		}
		ep, err := clientfunk.ZeroconfManagementLookup(config.ClusterName)
		if err != nil {
			fmt.Printf("Unable to do zeroconf lookup: %v\n", err)
			return
		}
		config.GRPCClient.ServerEndpoint = ep
	}

	if config.GRPCClient.ServerEndpoint == "" {
		fmt.Println("Need an endpoint for one of the cluster nodes")
		return
	}
	opts, err := toolbox.GetGRPCDialOpts(config.GRPCClient)
	if err != nil {
		fmt.Printf("Could not create GRPC dial options: %v\n", err)
		return
	}
	conn, err := grpc.Dial(config.GRPCClient.ServerEndpoint, opts...)
	if err != nil {
		fmt.Printf("Could not dial management endpoint: %v\n", err)
		return
	}
	client := clustermgmt.NewClusterManagementClient(conn)

	var runCommand commandRunner
	switch config.Command {
	case cmdGetState:
		runCommand = getState
	case cmdListNodes:
		runCommand = listNodes
	default:
		fmt.Printf("Unknown command: %s\n", config.Command)
		return
	}

	runCommand(client, config)
}

func getState(client clustermgmt.ClusterManagementClient, config parameters) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	res, err := client.GetState(ctx, &clustermgmt.GetStateRequest{})
	if err != nil {
		fmt.Printf("Error callling GetState on node: %v\n", err)
		return
	}

	fmt.Println("------------------------------------------------------------------------------")
	fmt.Printf("Node ID:    %s\n", res.NodeId)
	fmt.Printf("State: %s\n", res.State.String())
	fmt.Printf("Nodes: %d\n", res.NodeCount)
	fmt.Println("------------------------------------------------------------------------------")
}

func listNodes(client clustermgmt.ClusterManagementClient, config parameters) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	_, err := client.ListNodes(ctx, &clustermgmt.ListNodesRequest{})
	if err != nil {
		fmt.Printf("Error callling ListNodes on node: %v\n", err)
		return
	}

}

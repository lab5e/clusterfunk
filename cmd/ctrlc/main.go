package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/stalehd/clusterfunk/funk"
	"github.com/stalehd/clusterfunk/funk/clustermgmt"
	"github.com/stalehd/clusterfunk/toolbox"
)

type parameters struct {
	ClusterName   string `param:"desc=Cluster name;default=demo"`
	SerfNode      string `param:"desc=Serf node to attach to;default="`
	Zeroconf      bool   `param:"desc=Use zeroconf discovery for Serf;default=true"`
	ConnectLeader bool   `param:"desc=Attempt to connect to the leader node in the Raft cluster;default=false"`
	Command       string `param:"desc=Command to execute;option=get-state,list-ndoes"`
	ShowInactive  bool   `param:"desc=Show inactive nodes;default=false"`
	GRPCClient    toolbox.GRPCClientParam
}

const (
	cmdGetState  = "get-state"
	cmdListNodes = "list-nodes"
)

type commandRunner func(clustermgmt.ClusterManagementClient, parameters)

func main() {
	var config parameters
	flag.StringVar(&config.ClusterName, "cluster-name", "demo", "Name of cluster to connect with")
	flag.BoolVar(&config.Zeroconf, "zeroconf", true, "Use Zeroconf discovery")
	flag.StringVar(&config.SerfNode, "serf-node", "", "Serf node to use when joining cluster")
	flag.StringVar(&config.GRPCClient.ServerEndpoint, "server-endpoint", "", "gRPC server endpoint (bypass Serf lookup)")
	flag.StringVar(&config.Command, "cmd", "get-state", "Command to execute (get-state, list-serf, list-raft)")
	flag.BoolVar(&config.ShowInactive, "show-inactive", false, "Show inactive nodes in lists")
	flag.Parse()

	if config.Zeroconf && config.ClusterName == "" {
		fmt.Printf("Needs a cluster name if zeroconf is to be used for discovery")
		return
	}

	if config.SerfNode == "" && config.GRPCClient.ServerEndpoint == "" && !config.Zeroconf {
		fmt.Printf("Needs either zeroconf discovery, a serf node to connect to or a gRPC endpoint")
		return
	}
	var err error

	start := time.Now()
	if config.Zeroconf && config.SerfNode == "" {
		config.SerfNode, err = findSerfNode(config.ClusterName)
		if err != nil {
			fmt.Printf("Unable to locate a Serf node: %v\n", err)
			return
		}
	}
	end := time.Now()
	fmt.Printf("%f ms to look up\n", float64(end.Sub(start))/float64(time.Millisecond))

	start = time.Now()
	if config.SerfNode != "" && config.GRPCClient.ServerEndpoint == "" {
		config.GRPCClient.ServerEndpoint, err = findRaftNode(config.SerfNode)
		if err != nil {
			fmt.Printf("Unable to locate Raft Node: %v\n", err)
			return
		}
	}
	end = time.Now()
	fmt.Printf("%f ms to set up\n", float64(end.Sub(start))/float64(time.Millisecond))
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

	start = time.Now()
	client, err := connectToRaftNode(config.GRPCClient)
	end = time.Now()
	fmt.Printf("%f ms to connect\n", float64(end.Sub(start))/float64(time.Millisecond))
	if err != nil {
		fmt.Printf("Unable to connect to the raft node at %s: %v\n", config.GRPCClient.ServerEndpoint, err)
		return
	}

	if client == nil {
		return
	}
	start = time.Now()
	runCommand(client, config)
	end = time.Now()
	fmt.Printf("%f ms to execute\n", float64(end.Sub(start))/float64(time.Millisecond))
}

func findSerfNode(clusterName string) (string, error) {
	zr := toolbox.NewZeroconfRegistry(clusterName)
	nodes, err := zr.ResolveFirst(1 * time.Second)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", errors.New("No Serf nodes found")
	}
	return nodes, nil
}

func randomHostPort() string {
	addr, err := toolbox.FindPublicIPv4()
	if err != nil {
		return ":0"
	}
	port, _ := toolbox.FreeUDPPort()
	return fmt.Sprintf("%s:%d", addr, port)
}

// findRaftNode uses the serf endpoint to find a raft node
func findRaftNode(joinEndpoint string) (string, error) {

	serfNode := funk.NewSerfNode()
	if err := serfNode.Start(toolbox.RandomID(), false, funk.SerfParameters{
		Endpoint:    randomHostPort(),
		JoinAddress: joinEndpoint,
	}); err != nil {
		return "", err
	}
	defer serfNode.Stop()

	for _, v := range serfNode.Members() {
		if ep, exists := v.Tags[funk.ManagementEndpoint]; exists {
			return ep, nil
		}
	}
	return "", errors.New("no nodes with management endpoint found")
}

func connectToRaftNode(config toolbox.GRPCClientParam) (clustermgmt.ClusterManagementClient, error) {

	opts, err := toolbox.GetGRPCDialOpts(config)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(config.ServerEndpoint, opts...)
	if err != nil {
		return nil, err
	}
	return clustermgmt.NewClusterManagementClient(conn), nil
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

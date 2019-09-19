package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/stalehd/clusterfunk/cluster"
	"github.com/stalehd/clusterfunk/cluster/clustermgmt"
	"github.com/stalehd/clusterfunk/utils"
)

type parameters struct {
	ClusterName   string `param:"desc=Cluster name;default=demo"`
	SerfNode      string `param:"desc=Serf node to attach to;default="`
	Zeroconf      bool   `param:"desc=Use zeroconf discovery for Serf;default=true"`
	ConnectLeader bool   `param:"desc=Attempt to connect to the leader node in the Raft cluster;default=false"`
	Command       string `param:"desc=Command to execute;option=get-state,list-serf-list-raft"`
	ShowInactive  bool   `param:"desc=Show inactive nodes;default=false"`
	GRPCClient    utils.GRPCClientParam
}

const (
	cmdGetState = "get-state"
	cmdListSerf = "list-serf"
	cmdListRaft = "list-raft"
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

	if config.Zeroconf && config.SerfNode == "" {
		config.SerfNode, err = findSerfNode(config.ClusterName)
		if err != nil {
			fmt.Printf("Unable to locate a Serf node: %v\n", err)
			return
		}
	}
	if config.SerfNode != "" && config.GRPCClient.ServerEndpoint == "" {
		config.GRPCClient.ServerEndpoint, err = findRaftNode(config.SerfNode)
		if err != nil {
			fmt.Printf("Unable to locate Raft Node: %v\n", err)
			return
		}
	}

	var runCommand commandRunner
	switch config.Command {
	case cmdGetState:
		runCommand = getState
	case cmdListRaft:
		runCommand = listRaft
	case cmdListSerf:
		runCommand = listSerf
	default:
		fmt.Printf("Unknown command: %s\n", config.Command)
		return
	}

	client, err := connectToRaftNode(config.GRPCClient)
	if err != nil {
		fmt.Printf("Unable to connect to the raft node at %s: %v\n", config.GRPCClient.ServerEndpoint, err)
		return
	}

	if client == nil {
		return
	}

	runCommand(client, config)
}

func findSerfNode(clusterName string) (string, error) {
	zr := cluster.NewZeroconfRegistry(clusterName)
	nodes, err := zr.Resolve(1 * time.Second)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", errors.New("No Serf nodes found")
	}
	return nodes[0], nil
}

func randomHostPort() string {
	addr, err := utils.FindPublicIPv4()
	if err != nil {
		return ":0"
	}
	port, _ := utils.FreeUDPPort()
	return fmt.Sprintf("%s:%d", addr, port)
}

// findRaftNode uses the serf endpoint to find a raft node
func findRaftNode(joinEndpoint string) (string, error) {

	serfNode := cluster.NewSerfNode()
	serfNode.SetTag(cluster.NodeType, cluster.NonvoterKind)
	if err := serfNode.Start(utils.RandomID(), false, cluster.SerfParameters{
		Endpoint:    randomHostPort(),
		JoinAddress: joinEndpoint,
	}); err != nil {
		return "", err
	}
	defer serfNode.Stop()

	for _, v := range serfNode.Members() {
		if ep, exists := v.Tags[cluster.ManagementEndpoint]; exists {
			return ep, nil
		}
	}
	return "", errors.New("no nodes with management endpoint found")
}

func connectToRaftNode(config utils.GRPCClientParam) (clustermgmt.ClusterManagementClient, error) {

	opts, err := utils.GetGRPCDialOpts(config)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(config.ServerEndpoint, opts...)
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
	fmt.Printf("Raft state: %s\n", res.RaftState)
	fmt.Printf("Raft nodes: %d\n", res.RaftNodeCount)
	fmt.Printf("Serf nodes: %d\n", res.SerfNodeCount)
	fmt.Println("------------------------------------------------------------------------------")
}

func listRaft(client clustermgmt.ClusterManagementClient, config parameters) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	res, err := client.ListRaftNodes(ctx, &clustermgmt.ListRaftNodesRequest{})
	if err != nil {
		fmt.Printf("Error callling ListRaftNodes on node: %v\n", err)
		return
	}

	fmt.Println("------------------------------------------------------------------------------")
	fmt.Printf("Node ID:    %s\n", res.NodeId)
	fmt.Printf("%20s%15s%20s\n", "ID", "Raft state", "Is leader")
	for _, v := range res.Members {
		fmt.Printf("%20s%15s%20t\n", v.Id, v.RaftState, v.IsLeader)
	}
	fmt.Println("------------------------------------------------------------------------------")
}

func listSerf(client clustermgmt.ClusterManagementClient, config parameters) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	res, err := client.ListSerfNodes(ctx, &clustermgmt.ListSerfNodesRequest{})
	if err != nil {
		fmt.Printf("Error callling ListSerfNodes on node: %v\n", err)
		return
	}

	fmt.Println("------------------------------------------------------------------------------")
	fmt.Printf("Node ID:    %s\n", res.NodeId)
	fmt.Printf("%-20s%-25s%-15s%-20s%s\n", "ID", "Endpoint", "Status", "Name", "Host/port")
	for _, v := range res.Swarm {
		if !config.ShowInactive {
			if v.Status == "left" {
				continue
			}
		}
		fmt.Printf("%-20s%-25s%-15s\n", v.Id, v.Endpoint, v.Status)
		for _, e := range v.ServiceEndpoints {
			// this is just padding
			fmt.Printf("%60s%-20s%s\n", "", e.Name, e.HostPort)
		}
		fmt.Printf("%-80s%s\n", "", "Attributes")
		for _, e := range v.Attributes {
			// this is just padding
			fmt.Printf("%60s%-20s: %s\n", "", e.Name, e.Value)
		}
	}
}

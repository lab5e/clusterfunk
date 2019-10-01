package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/stalehd/clusterfunk/cluster"
	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/utils"
)

const numShards = 10000

func waitForExit() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	for {
		select {
		case <-terminate:
			return
		}
	}
}

func main() {
	var config cluster.Parameters
	flag.StringVar(&config.Serf.JoinAddress, "join", "", "Join address for cluster")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.BoolVar(&config.Raft.DiskStore, "disk", false, "Use disk store")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.ClusterName, "name", "demo", "Name of cluster")
	flag.BoolVar(&config.AutoJoin, "autojoin", true, "Autojoin via Serf Events")
	flag.Parse()

	shards := sharding.NewShardManager()
	if err := shards.Init(numShards, nil); err != nil {
		panic(err)
	}

	if err := start(config); err != nil {
		panic(err)
	}

	waitForExit()
}

var serfNode *cluster.SerfNode
var raftNode *cluster.RaftNode
var registry *utils.ZeroconfRegistry

func start(config cluster.Parameters) error {
	config.Final()
	if config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	serfNode = cluster.NewSerfNode()

	if config.ZeroConf {
		registry = utils.NewZeroconfRegistry(config.ClusterName)

		if !config.Raft.Bootstrap && config.Serf.JoinAddress == "" {
			var err error
			addrs, err := registry.Resolve(1 * time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			config.Serf.JoinAddress = addrs[0]
		}
		if err := registry.Register(config.NodeID, utils.PortOfHostPort(config.Serf.Endpoint)); err != nil {
			return err
		}

	}
	raftNode = cluster.NewRaftNode()

	go raftEvents(raftNode.Events())

	if err := raftNode.Start(config.NodeID, config.Verbose, config.Raft); err != nil {
		return err
	}

	serfNode.SetTag(cluster.RaftEndpoint, raftNode.Endpoint())
	serfNode.SetTag(cluster.SerfEndpoint, config.Serf.Endpoint)

	go serfEvents(serfNode.Events())

	if err := serfNode.Start(config.NodeID, config.Verbose, config.Serf); err != nil {
		return err
	}
	log.Printf("Starting")
	return nil
}

func raftEvents(ch <-chan cluster.RaftEventType) {
	for e := range ch {
		log.Printf("RAFT: %s", e.String())
		switch e {
		case cluster.RaftClusterSizeChanged:
			log.Printf("%d members:  %+v ", raftNode.MemberCount(), raftNode.Members())
		case cluster.RaftLeaderLost:
		case cluster.RaftBecameLeader:
		case cluster.RaftBecameFollower:
		case cluster.RaftReceivedLog:
		default:
			log.Printf("Unknown event received: %+v", e)
		}
	}
}

func serfEvents(ch <-chan cluster.NodeEvent) {
	for ev := range ch {
		if ev.Joined {
			if raftNode.Leader() {
				if err := raftNode.AddClusterNode(ev.NodeID, ev.Tags[cluster.RaftEndpoint]); err != nil {
					log.Printf("Error adding member: %v - %+v", err, ev)
				}
			}
			continue
		}
		if raftNode.Leader() {
			if err := raftNode.RemoveClusterNode(ev.NodeID, ev.Tags[cluster.RaftEndpoint]); err != nil {
				log.Printf("Error removing member: %v - %+v", err, ev)
			}
		}
	}
}

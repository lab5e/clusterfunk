package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/cluster"
	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/toolbox"
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

	defer raftNode.Stop()
	waitForExit()
}

var serfNode *cluster.SerfNode
var raftNode *cluster.RaftNode
var registry *toolbox.ZeroconfRegistry

func start(config cluster.Parameters) error {
	config.Final()
	if config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	serfNode = cluster.NewSerfNode()

	if config.ZeroConf {
		registry = toolbox.NewZeroconfRegistry(config.ClusterName)

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
		if err := registry.Register(config.NodeID, toolbox.PortOfHostPort(config.Serf.Endpoint)); err != nil {
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
	log.Info("Starting")
	return nil
}

func raftEvents(ch <-chan cluster.RaftEventType) {
	for e := range ch {
		log.WithField("event", e.String()).Info("raft event")
		switch e {
		case cluster.RaftClusterSizeChanged:
			log.WithFields(log.Fields{
				"size":    raftNode.Nodes.Size(),
				"members": raftNode.Nodes.List(),
			}).Info("Cluster")
		case cluster.RaftLeaderLost:
		case cluster.RaftBecameLeader:
		case cluster.RaftBecameFollower:
		case cluster.RaftReceivedLog:
		default:
			log.WithField("event", e).Info("Unknown event received")
		}
	}
}

func serfEvents(ch <-chan cluster.NodeEvent) {
	for ev := range ch {
		if ev.Joined {
			if raftNode.Leader() {
				if err := raftNode.AddClusterNode(ev.NodeID, ev.Tags[cluster.RaftEndpoint]); err != nil {
					log.WithError(err).WithField("member", ev.NodeID).Error("Error adding member")
				}
			}
			continue
		}
		if raftNode.Leader() {
			if err := raftNode.RemoveClusterNode(ev.NodeID, ev.Tags[cluster.RaftEndpoint]); err != nil {
				log.WithError(err).WithField("member", ev.NodeID).Error("Error removing member")
			}
		}
	}
}

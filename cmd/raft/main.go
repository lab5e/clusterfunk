package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/netutils"
	"github.com/sirupsen/logrus"
)

const numShards = 10000

func waitForExit() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}

func main() {
	var config funk.Parameters
	ja := ""
	flag.StringVar(&ja, "join", "", "Join address for cluster")
	config.Serf.JoinAddress = strings.Split(ja, ",")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.StringVar(&config.NodeID, "node-id", "", "Node ID")
	flag.StringVar(&config.Raft.DiskStore, "disk", "", "Persist log to disk")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.Name, "name", "demo", "Name of cluster")
	flag.BoolVar(&config.AutoJoin, "autojoin", true, "Autojoin via Serf Events")
	flag.Parse()

	shards := sharding.NewShardMap()
	if err := shards.Init(numShards); err != nil {
		panic(err)
	}

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})

	checker = funk.NewLivenessChecker(3)
	go func(ev <-chan string) {
		for id := range ev {
			logrus.WithField("id", id).Info("Client died")
		}
	}(checker.DeadEvents())
	if err := start(config); err != nil {
		panic(err)
	}

	defer func() {
		if err := raftNode.Stop(config.AutoJoin); err != nil {
			logrus.WithError(err).Info("Got error when stopping Raft node. Ignoring it since I'm shutting down.")
		}
	}()
	waitForExit()
}

var serfNode *funk.SerfNode
var raftNode *funk.RaftNode
var registry *toolbox.ZeroconfRegistry

var checker funk.LivenessChecker

const livenessEndpoint = "ep.live"

func start(config funk.Parameters) error {
	config.Final()
	if config.Name == "" {
		return errors.New("cluster name not specified")
	}

	localLiveEndpoint := netutils.RandomPublicEndpoint()

	serfNode = funk.NewSerfNode()
	serfNode.SetTag(livenessEndpoint, localLiveEndpoint)

	funk.NewLivenessClient(localLiveEndpoint)

	if config.ZeroConf {
		registry = toolbox.NewZeroconfRegistry(config.Name)

		if !config.Raft.Bootstrap && len(config.Serf.JoinAddress) == 0 {
			var err error
			addrs, err := registry.Resolve("serf", 1*time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			config.Serf.JoinAddress = addrs
		}
		if err := registry.Register("serf", config.NodeID, netutils.PortOfHostPort(config.Serf.Endpoint)); err != nil {
			return err
		}

	}
	raftNode = funk.NewRaftNode()

	go raftEvents(raftNode.Events())

	if _, err := raftNode.Start(config.NodeID, config.Raft); err != nil {
		return err
	}

	serfNode.SetTag(funk.RaftEndpoint, raftNode.Endpoint())
	serfNode.SetTag(funk.SerfEndpoint, config.Serf.Endpoint)

	go serfEvents(serfNode.Events())

	if err := serfNode.Start(config.NodeID, "", config.Serf); err != nil {
		return err
	}
	logrus.Info("Starting")
	return nil
}

func raftEvents(ch <-chan funk.RaftEventType) {
	for e := range ch {
		logrus.WithField("event", e.String()).Info("raft event")
		switch e {
		case funk.RaftClusterSizeChanged:
			logrus.WithFields(logrus.Fields{
				"size":    raftNode.Nodes.Size(),
				"members": raftNode.Nodes.List(),
			}).Info("Cluster")
			// Launch new liveness checks
			checker.Clear()
			for _, v := range raftNode.Nodes.List() {
				checker.Add(v, serfNode.Node(v).Tags[livenessEndpoint])
			}
		case funk.RaftLeaderLost:
			// Stop liveness check
			checker.Clear()

		case funk.RaftBecameLeader:
			// Start liveness check
			for _, v := range raftNode.Nodes.List() {
				checker.Add(v, serfNode.Node(v).Tags[livenessEndpoint])
			}

		case funk.RaftBecameFollower:
			// Stop liveness check
			checker.Clear()
		case funk.RaftReceivedLog:
		default:
			logrus.WithField("event", e).Info("Unknown event received")
		}
	}
}

func serfEvents(ch <-chan funk.NodeEvent) {
	for ev := range ch {
		switch ev.Event {
		case funk.SerfNodeJoined:
			if raftNode.Leader() {
				if err := raftNode.AddClusterNode(ev.Node.NodeID, ev.Node.Tags[funk.RaftEndpoint]); err != nil {
					logrus.WithError(err).WithField("member", ev.Node.NodeID).Error("Error adding member")
				}
			}
			continue
		case funk.SerfNodeLeft:
			if raftNode.Leader() {
				if err := raftNode.RemoveClusterNode(ev.Node.NodeID, ev.Node.Tags[funk.RaftEndpoint]); err != nil {
					logrus.WithError(err).WithField("member", ev.Node.NodeID).Error("Error removing member")
				}
			}
			// Ignoring updates and failed nodes. Failed nodes are handled by Raft. Updates aren't used
		}
	}
}

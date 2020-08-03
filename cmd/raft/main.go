package main

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
)

const numShards = 10000

func waitForExit() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}

func main() {
	var config funk.Parameters
	flag.StringVar(&config.Serf.JoinAddress, "join", "", "Join address for cluster")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.BoolVar(&config.Raft.DiskStore, "disk", false, "Use disk store")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.Name, "name", "demo", "Name of cluster")
	flag.BoolVar(&config.AutoJoin, "autojoin", true, "Autojoin via Serf Events")
	flag.Parse()

	shards := sharding.NewShardMap()
	if err := shards.Init(numShards, nil); err != nil {
		panic(err)
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})

	checker = funk.NewLivenessChecker(10*time.Millisecond, 3)
	go func(ev <-chan string) {
		for id := range ev {
			log.WithField("id", id).Info("Client died")
		}
	}(checker.DeadEvents())
	if err := start(config); err != nil {
		panic(err)
	}

	defer func() {
		if err := raftNode.Stop(config.AutoJoin); err != nil {
			log.WithError(err).Info("Got error when stopping Raft node. Ignoring it since I'm shutting down.")
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

	localLiveEndpoint := toolbox.RandomPublicEndpoint()

	serfNode = funk.NewSerfNode()
	serfNode.SetTag(livenessEndpoint, localLiveEndpoint)

	funk.NewLivenessClient(localLiveEndpoint)

	if config.ZeroConf {
		registry = toolbox.NewZeroconfRegistry(config.Name)

		if !config.Raft.Bootstrap && config.Serf.JoinAddress == "" {
			var err error
			addrs, err := registry.Resolve("serf", 1*time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			config.Serf.JoinAddress = addrs[0]
		}
		if err := registry.Register("serf", config.NodeID, toolbox.PortOfHostPort(config.Serf.Endpoint)); err != nil {
			return err
		}

	}
	raftNode = funk.NewRaftNode()

	go raftEvents(raftNode.Events())

	if err := raftNode.Start(config.NodeID, config.Raft); err != nil {
		return err
	}

	serfNode.SetTag(funk.RaftEndpoint, raftNode.Endpoint())
	serfNode.SetTag(funk.SerfEndpoint, config.Serf.Endpoint)

	go serfEvents(serfNode.Events())

	if err := serfNode.Start(config.NodeID, config.Serf); err != nil {
		return err
	}
	log.Info("Starting")
	return nil
}

func raftEvents(ch <-chan funk.RaftEventType) {
	for e := range ch {
		log.WithField("event", e.String()).Info("raft event")
		switch e {
		case funk.RaftClusterSizeChanged:
			log.WithFields(log.Fields{
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
			log.WithField("event", e).Info("Unknown event received")
		}
	}
}

func serfEvents(ch <-chan funk.NodeEvent) {
	for ev := range ch {
		switch ev.Event {
		case funk.SerfNodeJoined:
			if raftNode.Leader() {
				if err := raftNode.AddClusterNode(ev.Node.NodeID, ev.Node.Tags[funk.RaftEndpoint]); err != nil {
					log.WithError(err).WithField("member", ev.Node.NodeID).Error("Error adding member")
				}
			}
			continue
		case funk.SerfNodeLeft:
			if raftNode.Leader() {
				if err := raftNode.RemoveClusterNode(ev.Node.NodeID, ev.Node.Tags[funk.RaftEndpoint]); err != nil {
					log.WithError(err).WithField("member", ev.Node.NodeID).Error("Error removing member")
				}
			}
			// Ignoring updates and failed nodes. Failed nodes are handled by Raft. Updates aren't used
		}
	}
}

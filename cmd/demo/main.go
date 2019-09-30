package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/stalehd/clusterfunk/cluster/sharding"

	"github.com/stalehd/clusterfunk/cluster"
)

const numShards = 10000

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

	c := cluster.NewCluster(config, shards)
	defer c.Stop()
	/*
		go func(ch <-chan cluster.Event) {
			for ev := range ch {
				switch ev.LocalState {
				case cluster.Invalid:
					log.Println("DEMO STATE Cluster node is in invalid state")
				case cluster.Joining:
					log.Println("DEMO STATE Cluster node is joining a cluster")
				case cluster.Voting:
					log.Println("DEMO STATE Cluster node is voting")
				case cluster.Operational:
					log.Println("DEMO STATE Cluster node is operational")
				case cluster.Resharding:
					log.Println("DEMO STATE Cluster node is resharding")
				case cluster.Starting:
					log.Println("DEMO STATE Cluster node is starting")
				case cluster.Stopping:
					log.Println("DEMO STATE Cluster node is operational")
				default:
					log.Println("DEMO STATE *** Unknown state", ev.LocalState)
				}
			}
		}(c.Events())*/

	if err := c.Start(); err != nil {
		log.Printf("Error starting cluster: %v\n", err)
		return
	}

	waitForExit(c)
	log.Println("I'm done")
}

func waitForExit(c cluster.Cluster) {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	for {
		select {
		case <-terminate:
			return
		}
	}
}

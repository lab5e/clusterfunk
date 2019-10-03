package main

import (
	"flag"
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/cluster/sharding"

	"github.com/stalehd/clusterfunk/cluster"
)

const numShards = 10000

func main() {
	ll := "info"
	var config cluster.Parameters
	flag.StringVar(&config.Serf.JoinAddress, "join", "", "Join address for cluster")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.BoolVar(&config.Raft.DiskStore, "disk", false, "Use disk store")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.ClusterName, "name", "demo", "Name of cluster")
	flag.BoolVar(&config.AutoJoin, "autojoin", true, "Autojoin via Serf Events")
	flag.StringVar(&ll, "loglevel", "debug", "Logging level")
	flag.Parse()

	switch ll {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	}
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})
	shards := sharding.NewShardManager()
	if err := shards.Init(numShards, nil); err != nil {
		panic(err)
	}

	c := cluster.NewCluster(config, shards)
	defer c.Stop()

	go func(ch <-chan cluster.Event) {
		for ev := range ch {
			log.Infof("Cluster state: %s  role: %s", ev.State.String(), ev.Role.String())
		}
	}(c.Events())

	if err := c.Start(); err != nil {
		log.WithError(err).Error("Error starting cluster")
		return
	}

	waitForExit(c)
	log.Info("I'm done")
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

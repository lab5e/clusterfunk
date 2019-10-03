package main

import (
	"flag"

	golog "log"

	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/toolbox"

	"github.com/stalehd/clusterfunk/cluster"
)

const numShards = 10000

const demoEndpoint = "ep.demo"

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
	flag.StringVar(&ll, "loglevel", "info", "Logging level")
	flag.Parse()

	defaultLogger := log.New()
	// This mutes the default logs. The default level is info
	defaultLogger.SetLevel(log.WarnLevel)
	defaultLogger.Formatter = &log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"}
	w := defaultLogger.Writer()
	defer w.Close()
	golog.SetOutput(w)

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
			if ev.State == cluster.Operational {
				printShardMap(shards, c, demoEndpoint)
			}
		}
	}(c.Events())

	// TODO: Add endpoint to cluster node, launch gRPC service
	// c.AddLocalEndpoint(demoEndpoint, grpcEndpoint)
	if err := c.Start(); err != nil {
		log.WithError(err).Error("Error starting cluster")
		return
	}
	c.SetEndpoint(demoEndpoint, "0.0.0.0:0")

	toolbox.WaitForCtrlC()
	log.Info("I'm done")
}

func printShardMap(shards sharding.ShardManager, c cluster.Cluster, endpoint string) {
	allShards := shards.Shards()
	myShards := 0
	for _, v := range allShards {
		if v.NodeID() == c.NodeID() {
			myShards++
		}
	}

	log.Info("--- Peer info ---")
	log.Infof("%d shards allocated to me (out of %d total)", myShards, len(allShards))
	for _, v := range shards.NodeList() {
		m := "  "
		if v == c.NodeID() {
			m = "->"
		}
		log.Infof("%s Node %15s is serving at %s", m, v, c.GetEndpoint(v, endpoint))
	}
	log.Infof("--- End ---")
}

package main

import (
	"flag"

	"github.com/stalehd/clusterfunk/funk"

	golog "log"

	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/funk/sharding"
	"github.com/stalehd/clusterfunk/toolbox"
)

const numShards = 10000

const demoEndpoint = "ep.demo"

var logLevel = "info"
var config funk.Parameters
var defaultLogger = log.New()
var cluster funk.Cluster
var shards sharding.ShardManager

func init() {
	flag.StringVar(&config.Serf.JoinAddress, "join", "", "Join address for cluster")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.BoolVar(&config.Raft.DiskStore, "disk", false, "Use disk store")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.ClusterName, "name", "demo", "Name of cluster")
	flag.BoolVar(&config.AutoJoin, "autojoin", true, "Autojoin via Serf Events")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level")
	flag.Parse()
	config.NodeID = toolbox.RandomID()
	config.Final()
}
func main() {
	// Set up the shard map.
	shards = sharding.NewShardManager()
	if err := shards.Init(numShards, nil); err != nil {
		panic(err)
	}

	cluster = funk.NewCluster(config, shards)

	setupLogging()

	// This is the demo gRPC service we'll run on each node.
	demoServerEndpoint := toolbox.RandomPublicEndpoint()

	// Set up the local gRPC server.
	liffServer := newLiffProxy(newLiffServer(cluster.NodeID()), shards, cluster, demoEndpoint)
	go startDemoServer(demoServerEndpoint, liffServer)

	// This logs a message every time the cluster changes state.
	go clusterEventListener(cluster.Events())

	// ...announce the endpoint. This is sent via Serf so this should be set before
	// starting the cluster. Serf updates are slower than the leader election so
	// the nodes might not pick up the endpoint until after the election. If we
	// set the endpoint before the cluster is started the serf client will announce
	// itself with the endpoint already populated.
	cluster.SetEndpoint(demoEndpoint, demoServerEndpoint)

	// ...and start the cluster node. If the bootstrap flag is set a new cluster
	// will be launched.
	if err := cluster.Start(); err != nil {
		log.WithError(err).Error("Error starting cluster")
		return
	}
	defer cluster.Stop()

	// Nothing blocks here so wait for an interrupt signal.
	toolbox.WaitForCtrlC()
}

func clusterEventListener(ch <-chan funk.Event) {
	for ev := range ch {
		log.Infof("Cluster state: %s  role: %s", ev.State.String(), ev.Role.String())
		if ev.State == funk.Operational {
			printShardMap(shards, cluster, demoEndpoint)
		}
	}
}

// This prints the shard map and nodes in the cluster with the endpoint for
// each node's gRPC service.
func printShardMap(shards sharding.ShardManager, c funk.Cluster, endpoint string) {
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

func setupLogging() {
	// This mutes the logs from the log package in go. The default log level
	// for these are "info" so anything logged by the default logger will be
	// muted.
	defaultLogger.SetLevel(log.WarnLevel)
	defaultLogger.Formatter = &log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"}
	w := defaultLogger.Writer()
	defer w.Close()
	golog.SetOutput(w)

	// Set log level for logrus. The default level is Debug. The demo client will
	// log everything at Info or above.
	switch logLevel {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})
}

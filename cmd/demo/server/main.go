package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/stalehd/clusterfunk/cmd/demo/server/grpcserver"

	golog "log"

	"github.com/ExploratoryEngineering/params"
	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/cmd/demo/server/http"
	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/funk/sharding"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

const numShards = 10000

const demoEndpoint = "ep.demo"

var logLevel = "info"
var config parameters
var defaultLogger = log.New()
var cluster funk.Cluster
var shards sharding.ShardManager
var webserverEndpoint string
var cpuprofiler string

type parameters struct {
	CPUProfilerFile string `param:"desc=Turn on profiling and store the profile data in a file"`
	Cluster         funk.Parameters
}

func main() {
	if err := params.NewEnvFlag(&config, os.Args[1:]); err != nil {
		fmt.Println(err.Error())
		return
	}

	if config.CPUProfilerFile != "" {
		f, err := os.Create(config.CPUProfilerFile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Set up the shard map.
	shards = sharding.NewShardManager()
	if err := shards.Init(numShards, nil); err != nil {
		panic(err)
	}
	cluster = funk.NewCluster(config.Cluster, shards)

	setupLogging()

	demoServerEndpoint := toolbox.RandomPublicEndpoint()
	webserverEndpoint = toolbox.RandomLocalEndpoint()

	http.StartWebserver(webserverEndpoint, cluster, shards)
	go grpcserver.StartDemoServer(demoServerEndpoint, demoEndpoint, cluster, shards)

	go func(ch <-chan funk.Event) {
		for ev := range ch {
			log.Infof("Cluster state: %s  role: %s", ev.State.String(), ev.Role.String())

			http.UpdateClusterStatus(cluster)

			if ev.State == funk.Operational {
				printShardMap(shards, cluster, demoEndpoint)
				http.ClusterOperational(cluster, shards)
			}
		}
	}(cluster.Events())

	cluster.SetEndpoint(demoEndpoint, demoServerEndpoint)
	cluster.SetEndpoint(http.ConsoleEndpoint, webserverEndpoint)

	if err := cluster.Start(); err != nil {
		log.WithError(err).Error("Error starting cluster")
		return
	}
	defer cluster.Stop()

	toolbox.WaitForCtrlC()
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

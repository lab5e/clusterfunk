package main

import (
	"fmt"
	"log"
	gohttp "net/http"
	"os"
	"runtime/pprof"

	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/cmd/demo/server/grpcserver"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/cmd/demo/server/http"
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/gotoolbox/netutils"
	"github.com/lab5e/gotoolbox/rest"
	gotoolbox "github.com/lab5e/gotoolbox/toolbox"
)

const numShards = 10000

const demoEndpointName = "ep.demo"

var logLevel = "info"
var config parameters
var defaultLogger = logrus.New()
var cluster funk.Cluster
var shards sharding.ShardMap
var webserverEndpoint string
var metricsEndpoint string

type parameters struct {
	CPUProfilerFile string          `kong:"help='Turn on profiling and store the profile data in a file'"`
	Cluster         funk.Parameters `kong:"embed,prefix='cluster-'"`
}

func main() {
	k, err := kong.New(&config, kong.Name("server"),
		kong.Description("Demo server"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: false,
		}))
	if err != nil {
		panic(err)
	}
	if _, err := k.Parse(os.Args[1:]); err != nil {
		k.FatalIfErrorf(err)
		return
	}

	if config.CPUProfilerFile != "" {
		f, err := os.Create(config.CPUProfilerFile)
		if err != nil {
			logrus.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(fmt.Sprintf("Error starting CPU profiler: %v", err))
		}
		defer pprof.StopCPUProfile()
	}

	// Set up the shard map.
	shards = sharding.NewShardMap()
	if err := shards.Init(numShards); err != nil {
		panic(err)
	}
	cluster, err = funk.NewCluster(config.Cluster, shards)
	if err != nil {
		panic(fmt.Sprintf("Could not start cluster: %v", err))
	}
	setupLogging()

	demoServerEndpoint := netutils.RandomPublicEndpoint()
	webserverEndpoint = netutils.RandomPublicEndpoint()
	metricsEndpoint = netutils.RandomPublicEndpoint()

	gohttp.Handle("/metrics", rest.AddCORSHeaders(promhttp.Handler().ServeHTTP))
	go func() {
		logrus.WithField("endpoint", metricsEndpoint).Info("Prometheus metrics endpoint starting")
		fmt.Println("Error serving metrics: ", gohttp.ListenAndServe(metricsEndpoint, nil))
	}()

	http.StartWebserver(webserverEndpoint, cluster, shards)
	go grpcserver.StartDemoServer(demoServerEndpoint, demoEndpointName, cluster, shards, config.Cluster.Metrics)

	go func(ch <-chan funk.Event) {
		for ev := range ch {
			logrus.Infof("Cluster state: %s  role: %s", ev.State.String(), ev.Role.String())

			http.UpdateClusterStatus(cluster)

			if ev.State == funk.Operational {
				printShardMap(shards, cluster, demoEndpointName)
				http.ClusterOperational(cluster, shards)
			}
		}
	}(cluster.Events())

	cluster.SetEndpoint(demoEndpointName, demoServerEndpoint)
	cluster.SetEndpoint(http.ConsoleEndpointName, webserverEndpoint)
	cluster.SetEndpoint(http.MetricsEndpointName, metricsEndpoint)
	if err := cluster.Start(); err != nil {
		logrus.WithError(err).Error("Error starting cluster")
		return
	}
	defer cluster.Stop()

	gotoolbox.WaitForSignal()
}

// This prints the shard map and nodes in the cluster with the endpoint for
// each node's gRPC service.
func printShardMap(shards sharding.ShardMap, c funk.Cluster, endpoint string) {
	allShards := shards.Shards()
	myShards := 0
	for _, v := range allShards {
		if v.NodeID() == c.NodeID() {
			myShards++
		}
	}

	logrus.Info("--- Peer info ---")
	logrus.Infof("%d shards allocated to me (out of %d total)", myShards, len(allShards))
	for _, v := range shards.NodeList() {
		m := "  "
		if v == c.NodeID() {
			m = "->"
		}
		logrus.Infof("%s Node %15s is serving at %s", m, v, c.GetEndpoint(v, endpoint))
	}
	logrus.Infof("--- End ---")
}

func setupLogging() {
	// This mutes the logs from the log package in go. The default log level
	// for these are "info" so anything logged by the default logger will be
	// muted.
	defaultLogger.SetLevel(logrus.WarnLevel)
	defaultLogger.Formatter = &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"}
	w := defaultLogger.Writer()
	defer w.Close()
	log.SetOutput(w)

	// Set log level for logrus. The default level is Debug. The demo client will
	// log everything at Info or above.
	switch logLevel {
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	}

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})
}

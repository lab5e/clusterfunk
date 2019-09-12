package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/stalehd/clattering/cluster"
)

func main() {
	var config cluster.Parameters
	flag.StringVar(&config.Join, "join", "", "Join address for cluster")
	flag.BoolVar(&config.Bootstrap, "bootstrap", false, "Bootstrap a new cluster")
	flag.BoolVar(&config.Loopback, "loopback", false, "Use loopback adapter")
	flag.BoolVar(&config.DiskStore, "disk", false, "Use disk store")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&config.ZeroConf, "zeroconf", true, "Use zeroconf (mDNS) to discover nodes")
	flag.StringVar(&config.ClusterName, "name", "demo", "Name of cluster")
	flag.Parse()
	c := cluster.NewCluster(config)
	defer c.Stop()

	if err := c.Start(); err != nil {
		log.Printf("Error starting cluster: %v\n", err)
		return
	}
	waitForExit(c)
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

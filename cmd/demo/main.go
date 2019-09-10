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
	flag.BoolVar(&config.Loopback, "loopback", true, "Use loopback adapter")
	flag.BoolVar(&config.DiskStore, "disk", false, "Use disk store")
	flag.Parse()
	c := cluster.New(config)
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

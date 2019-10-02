package cluster

import (
	log "github.com/sirupsen/logrus"

	"time"
)

// various debugging and diagnostic functions here.

func timeCall(f func(), desc string) {
	start := time.Now()
	f()
	end := time.Now()
	log.WithFields(log.Fields{
		"op": desc,
		"ms": float64(end.Sub(start)) / float64(time.Millisecond),
	}).Debug("Timed call")
}

// This is diagnostic functions. They can be removed.. Eventually

func (c *clusterfunkCluster) dumpShardMap() {
	log.Info("================== shard map ======================")
	nodes := make(map[string]int)
	for _, v := range c.shardManager.Shards() {
		n := nodes[v.NodeID()]
		n++
		nodes[v.NodeID()] = n
	}
	for k, v := range nodes {
		log.Infof("%-20s: %d shards", k, v)
	}
	log.Info("===================================================")
}

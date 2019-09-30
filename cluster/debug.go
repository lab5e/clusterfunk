package cluster

import (
	"log"
	"time"
)

// various debugging and diagnostic functions here.

func timeCall(f func(), desc string) {
	start := time.Now()
	f()
	end := time.Now()
	log.Printf("%s took %f ms", desc, float64(end.Sub(start))/float64(time.Millisecond))
}

// This is diagnostic functions. They can be removed.. Eventually

func (c *clusterfunkCluster) dumpShardMap() {
	log.Println("================== shard map ======================")
	nodes := make(map[string]int)
	for _, v := range c.shardManager.Shards() {
		n := nodes[v.NodeID()]
		n++
		nodes[v.NodeID()] = n
	}
	for k, v := range nodes {
		log.Printf("%-20s: %d shards", k, v)
	}
	log.Println("===================================================")
}

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

// Dump the node's world view
func (c *clusterfunkCluster) dumpNodes() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	log.Printf("==================== nodes ========================")
	for _, v := range c.nodes {
		me := ""
		if v.ID == c.raftNode.LocalNodeID() {
			me = "(that's me!)"
		}
		log.Printf("ID: %s %s", v.ID, me)
		for k, v := range v.Tags {
			log.Printf("           %s = %s", k, v)
		}
		log.Printf("- - - - - - - - - - - - - - - - - - - - - - - - - -")
	}
}

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

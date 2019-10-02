package cluster

import (
	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/cluster/sharding"
)

// This is diagnostic functions. They can be removed.. Eventually

func dumpShardMap(shardManager sharding.ShardManager) {
	log.Info("================== shard map ======================")
	nodes := make(map[string]int)
	for _, v := range shardManager.Shards() {
		n := nodes[v.NodeID()]
		n++
		nodes[v.NodeID()] = n
	}
	for k, v := range nodes {
		log.Infof("%-20s: %d shards", k, v)
	}
	log.Info("===================================================")
}

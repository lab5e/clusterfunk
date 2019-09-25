package sharding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// verifyDistribution ensures that the
func verifyDistribution(t *testing.T, manager ShardManager) {
	assert := require.New(t)

	nodes := make(map[string]int)
	shards := manager.Shards()

	for i := range shards {
		w := nodes[shards[i].NodeID()]
		nodes[shards[i].NodeID()] = w + shards[i].Weight()
	}

	totalWeight := 0
	for _, v := range nodes {
		totalWeight += v
	}

	assert.Equal(totalWeight, manager.TotalWeight(), "Manager reports incorrect total weight")

	weightPerNode := float64(totalWeight) / float64(len(nodes))

	for k, v := range nodes {
		t.Logf("  node: %s w: %d", k, v)
		assert.InDelta(v, weightPerNode, 0.1*weightPerNode, "Distribution is incorrect")
	}
}

// verifyShards ensures all shards are distributed to nodes
func verifyShards(t *testing.T, manager ShardManager, maxShards int) {
	assert := require.New(t)

	check := make(map[int]int)
	shards := manager.Shards()
	assert.Equal(maxShards, len(shards), "Incorrect length of shards")
	for i := range shards {
		check[shards[i].ID()] = 1
	}
	for i := 0; i < maxShards; i++ {
		assert.Contains(check, i, "Shard %d does not exist", i)
	}
}

func testShardManager(t *testing.T, manager ShardManager, maxShards int, weights []int) {
	assert := require.New(t)
	assert.Error(manager.Init(0, nil), "Expected error when maxShards = 0")
	assert.Error(manager.Init(len(weights), []int{}), "Expected error when weights != maxShards")

	assert.NoError(manager.Init(len(weights), weights), "Regular init should work")
	assert.Error(manager.Init(len(weights), weights), "Should not be allowed to init manager twice")

	manager.UpdateNodes("A")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("A", "B")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("A", "B", "C")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	for i := 0; i < maxShards; i++ {
		assert.NotEqual("", manager.MapToNode(i).NodeID(), "Shard %d is not mapped to a node", i)
	}

	manager.UpdateNodes("A", "C")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("C")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes([]string{}...)
	verifyDistribution(t, manager)

	require.Panics(t, func() { manager.MapToNode(-1) }, "Panics on invalid shard ID")
}

// These are sort-of-sensible defaults for benchmarks
const (
	benchmarkShardCount = 1000
	benchmarkNodeCount  = 9
)

package sharding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// verifyShards ensures all shards are distributed to nodes
func verifyShards(t *testing.T, manager ShardMap, maxShards int) {
	assert := require.New(t)
	check := make(map[int]int)
	shards := manager.Shards()
	assert.Equal(maxShards, len(shards), "Incorrect length of shards")
	expectedNodes := make(map[string]bool)
	for _, v := range manager.NodeList() {
		expectedNodes[v] = true
	}

	for i := range shards {
		check[shards[i].ID()] = 1
		assert.Containsf(expectedNodes, shards[i].NodeID(), "Shard %d does not contain a node in the node list (%s)", shards[i].ID(), shards[i].NodeID())
	}
	for i := 0; i < maxShards; i++ {
		assert.Contains(check, i, "Shard %d does not exist", i)
	}
}

func verifyWorkerIDs(t *testing.T, manager ShardMap) {
	assert := require.New(t)
	workers := make([]int, 0)
	for _, v := range manager.NodeList() {
		workerID := manager.WorkerID(v)
		assert.NotContains(workers, workerID)
		workers = append(workers, workerID)
	}
}

func testShardManager(t *testing.T, manager ShardMap, maxShards int) {
	assert := require.New(t)
	assert.Error(manager.Init(0), "Expected error when maxShards = 0")

	assert.NoError(manager.Init(maxShards), "Regular init should work")
	assert.Error(manager.Init(maxShards), "Should not be allowed to init manager twice")

	manager.UpdateNodes("A")
	assert.Len(manager.NodeList(), 1)
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes("B", "A")
	assert.Len(manager.NodeList(), 2)
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes("C", "B", "A")
	assert.Len(manager.NodeList(), 3)
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes("B", "A", "C", "D", "E")
	assert.Len(manager.NodeList(), 5, "Manager should contain 5 nodes")
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes("B", "C", "D")
	assert.Len(manager.NodeList(), 3, "Manager should contain 3 nodes")
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	for i := 0; i < maxShards; i++ {
		assert.NotEqual("", manager.MapToNode(i).NodeID(), "Shard %d is not mapped to a node", i)
	}

	manager.UpdateNodes("A", "C")
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes("C")
	verifyShards(t, manager, maxShards)
	verifyWorkerIDs(t, manager)

	manager.UpdateNodes([]string{}...)
	verifyWorkerIDs(t, manager)

	require.Panics(t, func() { manager.MapToNode(-1) }, "Panics on invalid shard ID")
}

// These are sort-of-sensible defaults for benchmarks
const (
	benchmarkShardCount = 1000
	benchmarkNodeCount  = 9
)

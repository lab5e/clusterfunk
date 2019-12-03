package sharding
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"testing"

	"github.com/stretchr/testify/require"
)

// verifyDistribution ensures that the
func verifyDistribution(t *testing.T, manager ShardMap) {
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
		assert.Contains(expectedNodes, shards[i].NodeID())
	}
	for i := 0; i < maxShards; i++ {
		assert.Contains(check, i, "Shard %d does not exist", i)
	}
}

func testShardManager(t *testing.T, manager ShardMap, maxShards int, weights []int) {
	assert := require.New(t)
	assert.Error(manager.Init(0, nil), "Expected error when maxShards = 0")
	assert.Error(manager.Init(len(weights), []int{}), "Expected error when weights != maxShards")

	assert.NoError(manager.Init(len(weights), weights), "Regular init should work")
	assert.Error(manager.Init(len(weights), weights), "Should not be allowed to init manager twice")

	manager.UpdateNodes("A")
	assert.Len(manager.NodeList(), 1)
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("B", "A")
	assert.Len(manager.NodeList(), 2)
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("C", "B", "A")
	assert.Len(manager.NodeList(), 3)
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("B", "A", "C", "D", "E")
	assert.Len(manager.NodeList(), 5, "Manager should contain 5 nodes")
	verifyShards(t, manager, maxShards)

	manager.UpdateNodes("B", "C", "D")
	assert.Len(manager.NodeList(), 3, "Manager should contain 3 nodes")
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

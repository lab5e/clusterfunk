package cluster

import (
	"testing"
)

// verifyDistribution ensures that the
func verifyDistribution(t *testing.T, manager ShardManager) {
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

	if totalWeight != manager.TotalWeight() {
		t.Fatalf("Shards total weight is %d but manager reports %d", totalWeight, manager.TotalWeight())
	}

	weightPerNode := float32(totalWeight) / float32(len(nodes))
	// Allow for 10% off. If there's less than 10 shards and 3 or more nodes the test will fail but
	// shards should be >>> nodes.
	minWeight := 0.9 * weightPerNode
	maxWeight := 1.1 * weightPerNode

	t.Logf("totalW=%d nodes=%d minW=%f weight=%f maxW=%f", totalWeight, len(nodes), minWeight, weightPerNode, maxWeight)
	for k, v := range nodes {
		t.Logf("  node: %s w: %d", k, v)
		if float32(v) < minWeight || float32(v) > maxWeight {
			t.Fatalf("Distribution is off. Expected %f < weight[%s] < %f but weight = %d", minWeight, k, maxWeight, v)
		}
	}
}

// verifyShards ensures all shards are distributed to nodes
func verifyShards(t *testing.T, manager ShardManager, maxShards int) {
	check := make(map[int]int)
	shards := manager.Shards()
	if len(shards) != maxShards {
		t.Fatalf("maxShards != shards (expecting %d but got %d)", maxShards, len(shards))
	}
	for i := range shards {
		check[shards[i].ID()] = 1
	}
	for i := 0; i < maxShards; i++ {
		_, exists := check[i]
		if !exists {
			t.Fatalf("Missing shard %d", i)
		}
	}
}
func verifyPanicOnInvalidShardID(t *testing.T, manager ShardManager) {
	defer func() {
		recover()
	}()
	manager.MapToNode(-1)
	t.Fatal("No panic on unknown shard")
}

func verifyPanicOnUnknownNodeRemoval(t *testing.T, manager ShardManager) {
	defer func() {
		recover()
	}()
	manager.RemoveNode("unknown")
	t.Fatal("No panic on unknown node")
}

func testShardManager(t *testing.T, manager ShardManager, maxShards int, weights []int) {
	if err := manager.Init(0, nil); err == nil {
		t.Fatal("Expected error when maxShards is 0")
	}
	if err := manager.Init(len(weights), []int{}); err == nil {
		t.Fatal("Expected error when weights != maxShards")
	}
	if err := manager.Init(len(weights), weights); err != nil {
		t.Fatal(err)
	}
	if err := manager.Init(len(weights), weights); err == nil {
		t.Fatal("Should not be allowed to init manager twice")
	}
	manager.AddNode("A")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)
	manager.AddNode("B")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)
	manager.AddNode("C")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)

	for i := 0; i < maxShards; i++ {
		if manager.MapToNode(i).NodeID() == "" {
			t.Fatalf("Shard %d is not mapped to a node", i)
		}
	}
	manager.RemoveNode("B")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)
	manager.RemoveNode("A")
	verifyDistribution(t, manager)
	verifyShards(t, manager, maxShards)
	manager.RemoveNode("C")
	verifyDistribution(t, manager)

	verifyPanicOnInvalidShardID(t, manager)
	verifyPanicOnUnknownNodeRemoval(t, manager)
}

// These are sort-of-sensible defaults for benchmarks
const (
	benchmarkShardCount = 10000
	benchmarkNodeCount  = 50
)

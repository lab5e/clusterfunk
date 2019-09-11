package cluster

import (
	"testing"
)

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

	for k, v := range nodes {
		if float32(v) < minWeight || float32(v) > maxWeight {
			t.Fatalf("Distribution is off. Expected %f < weight[%s] < %f but weight = %d", minWeight, k, maxWeight, v)
		}
	}
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
	manager.AddNode("B")
	verifyDistribution(t, manager)
	manager.AddNode("C")
	verifyDistribution(t, manager)

	manager.RemoveNode("B")
	verifyDistribution(t, manager)
	manager.RemoveNode("A")
	verifyDistribution(t, manager)
	manager.RemoveNode("C")
	verifyDistribution(t, manager)
}

// These are sort-of-sensible defaults for benchmarks
const (
	benchmarkShardCount = 10000
	benchmarkNodeCount  = 50
)

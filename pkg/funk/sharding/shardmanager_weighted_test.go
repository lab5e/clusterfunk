package sharding

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeUpdate(t *testing.T) {
	assert := require.New(t)
	sm := NewShardMap()
	assert.NoError(sm.Init(10000, nil))

	sm.UpdateNodes("A")
	assert.Len(sm.NodeList(), 1)
	sm.UpdateNodes("B", "A")
	assert.Len(sm.NodeList(), 2)

	sm.UpdateNodes("C", "B", "A")
	assert.Len(sm.NodeList(), 3)

	sm.UpdateNodes("A", "B", "C", "D")
	assert.Len(sm.NodeList(), 4)

	sm.UpdateNodes("E", "A", "B", "C", "D")
	assert.Len(sm.NodeList(), 5)

	sm.UpdateNodes("C", "B", "A", "E", "D", "F")
	assert.Len(sm.NodeList(), 6)

	sm.UpdateNodes("A", "B", "C")
	assert.Len(sm.NodeList(), 3)

	sm.UpdateNodes("D", "A", "B", "C")
	assert.Len(sm.NodeList(), 4)
}

// TestNodeData is an internal test
func TestNodeData(t *testing.T) {
	nd := newNodeData("node1")
	nd.AddShard(NewShard(1, 1))
	nd.AddShard(NewShard(2, 2))
	nd.AddShard(NewShard(3, 3))
	nd.AddShard(NewShard(4, 4))

	if nd.TotalWeights != 10 {
		t.Fatal("Expected w=10")
	}
	s1 := nd.RemoveShard(1)
	if s1.Weight() != 1 {
		t.Fatal("expected weight 1")
	}
	if nd.TotalWeights != 9 {
		t.Fatal("Expected w=9")
	}
	s2 := nd.RemoveShard(2)
	if s2.Weight() != 2 {
		t.Fatalf("expected weight 2, got %+v", s2)
	}
	if nd.TotalWeights != 7 {
		t.Fatal("Expected w=7")
	}
	s3 := nd.RemoveShard(3)
	if s3.Weight() != 3 {
		t.Fatal("expected weight 3")
	}
	if nd.TotalWeights != 4 {
		t.Fatal("Expected w=4")
	}
	s4 := nd.RemoveShard(4)
	if s4.Weight() != 4 {
		t.Fatal("expected weight 4")
	}
	if nd.TotalWeights != 0 {
		t.Fatal("Expected w=0")
	}

	defer func() {
		// nolint
		recover()
	}()
	nd.RemoveShard(1)
	t.Fatal("no panic when zero shards left")
}

// TestWeightedShardManager tests the (default) shard manager
func TestWeightedShardManager(t *testing.T) {
	sm := NewShardMap()

	const maxShards = 1000
	weights := make([]int, maxShards)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	testShardManager(t, sm, maxShards, weights)
}

// Benchmark the performance on add and remove node
func BenchmarkWeightedShardManager(b *testing.B) {
	sm := NewShardMap()
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}

	// nolint - don't care about error returns here
	sm.Init(benchmarkShardCount, weights)

	var nodes []string
	for i := 0; i < benchmarkNodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("Node%04d", i))
	}
	sm.UpdateNodes(nodes...)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nodes = nodes[2:]
		nodes = append(nodes, fmt.Sprintf("%d", i))
		nodes = append(nodes, fmt.Sprintf("%db", i))
		sm.UpdateNodes(nodes...)
	}
}

// Benchmark lookups on node
func BenchmarkMapToNode(b *testing.B) {
	sm := NewShardMap()
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	// nolint - won't check error return
	sm.Init(benchmarkShardCount, weights)

	var nodes []string
	for i := 0; i < benchmarkNodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("Node%04d", i))
	}
	sm.UpdateNodes(nodes...)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sm.MapToNode(rand.Intn(benchmarkShardCount))
	}
}

// Benchmark init function
func BenchmarkShardInit(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm := NewShardMap()
		// nolint - won't check for errors
		sm.Init(benchmarkShardCount, weights)
	}
}

// Benchmark shard weight total (it should be *really* quick)
func BenchmarkShardWeight(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardMap()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.TotalWeight()
	}
}

func TestMarshalUnmarshalBinary(t *testing.T) {
	assert := require.New(t)
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardMap()

	assert.NoError(sm.Init(benchmarkShardCount, weights))

	var nodes []string
	for i := 0; i < benchmarkNodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("Node%04d", i))
	}
	sm.UpdateNodes(nodes...)
	buf, err := sm.MarshalBinary()
	assert.NoError(err)

	t.Logf("%d shards = %d bytes", benchmarkShardCount, len(buf))
	newManager := NewShardMap()
	assert.NoError(newManager.UnmarshalBinary(buf))

	assert.Equal(sm.TotalWeight(), newManager.TotalWeight(), "Total weight for both should be the same")

	assert.Equal(len(newManager.Shards()), len(sm.Shards()), "Number of shards should be the same")

	for i := 0; i < benchmarkShardCount; i++ {
		old := sm.MapToNode(i)
		new := newManager.MapToNode(i)
		assert.Equalf(new.NodeID(), old.NodeID(), "Shard %d is in a different place", i)
		assert.Equalf(new.Weight(), old.Weight(), "Shard %d has different weight", i)
		assert.Equalf(new.ID(), old.ID(), "Shard %d has different ID", i)
	}
}

func BenchmarkMarshalManager(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardMap()
	// nolint - wont't check error return
	sm.Init(benchmarkShardCount, weights)
	var nodes []string
	for i := 0; i < benchmarkNodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("Node%04d", i))
	}
	sm.UpdateNodes(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// nolint - won't check error returns
		sm.MarshalBinary()
	}
}

func BenchmarkUnmarshalManager(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardMap()
	// nolint - won't check error returns
	sm.Init(benchmarkShardCount, weights)
	nodes := []string{}
	for i := 0; i < benchmarkNodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("Node%04d", i))
	}
	sm.UpdateNodes(nodes...)
	buf, _ := sm.MarshalBinary()

	newManager := NewShardMap()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// nolint - this is a benchmark
		newManager.UnmarshalBinary(buf)
	}
}

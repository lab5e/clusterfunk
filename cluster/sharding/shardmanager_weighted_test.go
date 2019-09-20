package sharding

import (
	"fmt"
	"math/rand"
	"testing"
)

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
		recover()
	}()
	nd.RemoveShard(1)
	t.Fatal("no panic when zero shards left")
}

// TestWeightedShardManager tests the (default) shard manager
func TestWeightedShardManager(t *testing.T) {
	sm := NewShardManager()

	const maxShards = 1000
	weights := make([]int, maxShards)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	testShardManager(t, sm, maxShards, weights)
}

// Benchmark the performance on add and remove node
func BenchmarkWeightedShardManager(b *testing.B) {
	sm := NewShardManager()
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm.Init(benchmarkShardCount, weights)

	for i := 0; i < benchmarkNodeCount; i++ {
		sm.AddNode(fmt.Sprintf("Node%04d", i))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sm.AddNode("NodeXXX")
		sm.RemoveNode("NodeXXX")
	}
}

// Benchmark lookups on node
func BenchmarkMapToNode(b *testing.B) {
	sm := NewShardManager()
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm.Init(benchmarkShardCount, weights)

	for i := 0; i < benchmarkNodeCount; i++ {
		sm.AddNode(fmt.Sprintf("Node%04d", i))
	}
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
		sm := NewShardManager()
		sm.Init(benchmarkShardCount, weights)
	}
}

// Benchmark shard weight total (it should be *really* quick)
func BenchmarkShardWeight(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.TotalWeight()
	}
}

func TestMarshalUnmarshalBinary(t *testing.T) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardManager()
	sm.Init(benchmarkShardCount, weights)
	for i := 0; i < benchmarkNodeCount; i++ {
		sm.AddNode(fmt.Sprintf("Node%04d", i))
	}

	buf, err := sm.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d shards = %d bytes", benchmarkShardCount, len(buf))
	newManager := NewShardManager()
	if err := newManager.UnmarshalBinary(buf); err != nil {
		t.Fatal(err)
	}

	if newManager.TotalWeight() != sm.TotalWeight() {
		t.Fatalf("Total weight is different")
	}
	if len(newManager.Shards()) != len(sm.Shards()) {
		t.Fatalf("Number of shards is different: %d != %d", len(newManager.Shards()), len(sm.Shards()))
	}
	for i := 0; i < benchmarkShardCount; i++ {
		old := sm.MapToNode(i)
		new := newManager.MapToNode(i)
		if new.NodeID() != old.NodeID() {
			t.Fatalf("Shard %d is in a different place (%s/%s)", i, new.NodeID(), old.NodeID())
		}
		if new.Weight() != old.Weight() {
			t.Fatalf("Shard %d has different weight", i)
		}
		if new.ID() != old.ID() {
			t.Fatalf("Shard %d has different ID", i)
		}
	}
}

func BenchmarkMarshalManager(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardManager()
	sm.Init(benchmarkShardCount, weights)
	for i := 0; i < benchmarkNodeCount; i++ {
		sm.AddNode(fmt.Sprintf("Node%04d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.MarshalBinary()
	}
}

func BenchmarkUnmarshalManager(b *testing.B) {
	weights := make([]int, benchmarkShardCount)
	for i := range weights {
		weights[i] = int(rand.Int31n(100)) + 1
	}
	sm := NewShardManager()
	sm.Init(benchmarkShardCount, weights)
	for i := 0; i < benchmarkNodeCount; i++ {
		sm.AddNode(fmt.Sprintf("Node%04d", i))
	}

	buf, _ := sm.MarshalBinary()

	newManager := NewShardManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newManager.UnmarshalBinary(buf)
	}
}

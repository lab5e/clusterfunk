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
	assert.NoError(sm.Init(10000))

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
	assert := require.New(t)
	nd := newNodeData("node1")
	nd.AddShard(NewShard(1))
	nd.AddShard(NewShard(2))
	nd.AddShard(NewShard(3))
	nd.AddShard(NewShard(4))

	shardList := []int{1, 2, 3, 4}
	assert.Contains(shardList, nd.RemoveShard().ID())
	assert.Contains(shardList, nd.RemoveShard().ID())
	assert.Contains(shardList, nd.RemoveShard().ID())
	assert.Contains(shardList, nd.RemoveShard().ID())

	defer func() {
		// nolint
		recover()
	}()
	nd.RemoveShard()
	t.Fatal("no panic when zero shards left")
}

// TestSimpleShardManager tests the (default) shard manager
func TestSimpleShardManager(t *testing.T) {
	sm := NewShardMap()

	const maxShards = 1000
	testShardManager(t, sm, maxShards)
}

// Benchmark the performance on add and remove node
func BenchmarkWeightedShardManager(b *testing.B) {
	sm := NewShardMap()

	// nolint - don't care about error returns here
	sm.Init(benchmarkShardCount)

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
	// nolint - won't check error return
	sm.Init(benchmarkShardCount)

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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm := NewShardMap()
		// nolint - won't check for errors
		sm.Init(benchmarkShardCount)
	}
}

func TestMarshalUnmarshalBinary(t *testing.T) {
	assert := require.New(t)
	sm := NewShardMap()

	assert.NoError(sm.Init(benchmarkShardCount))

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

	assert.Equal(len(newManager.Shards()), len(sm.Shards()), "Number of shards should be the same")

	for i := 0; i < benchmarkShardCount; i++ {
		old := sm.MapToNode(i)
		new := newManager.MapToNode(i)
		assert.Equalf(new.NodeID(), old.NodeID(), "Shard %d is in a different place", i)
		assert.Equalf(new.ID(), old.ID(), "Shard %d has different ID", i)
	}
}

func TestNewAndOldSet(t *testing.T) {
	assert := require.New(t)

	oldSM := NewShardMap()
	assert.NoError(oldSM.Init(10))
	oldSM.UpdateNodes("node1")

	newSM := NewShardMap()
	assert.NoError(newSM.Init(10))
	newSM.UpdateNodes("node1", "node2")

	added := newSM.NewShards("node1", oldSM)
	assert.Len(added, 0)
	removed := newSM.DeletedShards("node1", oldSM)
	assert.Len(removed, 5)

}
func BenchmarkMarshalManager(b *testing.B) {
	sm := NewShardMap()
	// nolint - wont't check error return
	sm.Init(benchmarkShardCount)
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
	sm := NewShardMap()
	// nolint - won't check error returns
	sm.Init(benchmarkShardCount)
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

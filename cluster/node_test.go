package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeCollection(t *testing.T) {
	assert := assert.New(t)

	n := newNodeCollection()

	assert.True(n.AddNode("1"), "Node 1 does not exist")
	assert.True(n.AddNode("2"), "Node 2 does not exist")

	assert.Equal(2, n.Size(), "Size is 2")

	assert.False(n.AddNode("2"), "Node 1 already exists")
	assert.False(n.AddNode("1"), "Node 1 already exists")

	assert.Equal(2, n.Size(), "Size is still 2")

	assert.False(n.RemoveNode("4"), "Node 4 does not exist")

	assert.True(n.RemoveNode("1"), "Node 1 is removed")
	assert.Equal(1, n.Size(), "Size is 1")
	assert.False(n.RemoveNode(""), "Node does not contain \"\"")
	assert.True(n.RemoveNode("2"), "Node 2 is removed")

	// All this work just to make a silly naming joke. Oh my.
	assert.True(n.Sync("A", "B", "C", "D"), "It's n*synced")

	assert.False(n.Sync("A", "B", "C", "D"), "This isn't n*synced")

	assert.Equal(4, n.Size(), "Size is 4")
	assert.True(n.Sync("A", "B", "C", "D", "1"), "Is synced")
	assert.Equal(5, n.Size(), "Size is 5")

	// This is getting a bit too much. I'm truly sorry for this.
	assert.True(n.Sync("A", "1", "2", "3", "4"), "Tricky sync")
	assert.Equal(5, n.Size(), "Size should be 5")

}

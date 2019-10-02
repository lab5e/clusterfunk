package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeCollection(t *testing.T) {
	assert := assert.New(t)

	n := newNodeCollection()

	// All this work just to make a silly naming joke. Oh my.
	assert.True(n.Sync("A", "B", "C", "D"), "It's n*synced")

	assert.False(n.Sync("A", "B", "C", "D"), "This isn't n*synced")

	assert.Equal(4, n.Size(), "Size is 4")
	assert.True(n.Sync("A", "B", "C", "D", "1"), "Is synced")
	assert.Equal(5, n.Size(), "Size is 5")

	// This is getting a bit too much. I'm truly sorry for this.
	assert.True(n.Sync("A", "1", "2", "3", "4"), "Tricky sync")
	assert.Equal(5, n.Size(), "Size should be 5")

	assert.Contains(n.List(), "A")
	assert.Contains(n.List(), "2")
	assert.Contains(n.List(), "1")
	assert.Contains(n.List(), "3")
	assert.Contains(n.List(), "4")
}

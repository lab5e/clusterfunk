package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSetSync(t *testing.T) {
	assert := assert.New(t)

	n := NewStringSet()

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

	n.Clear()

	assert.Equal(n.Size(), 0)
	assert.Len(n.Strings, 0)

}

func TestAddRemoveStringSet(t *testing.T) {
	assert := assert.New(t)
	s := NewStringSet()
	assert.True(s.Add("1"))
	assert.Len(s.Strings, 1)
	assert.True(s.Add("2"))
	assert.Len(s.Strings, 2)

	assert.False(s.Add("1"))
	assert.Len(s.Strings, 2)

	assert.False(s.Remove("9"))
	assert.Len(s.Strings, 2)

	assert.True(s.Remove("1"))
	assert.Len(s.Strings, 1)
	assert.Contains(s.Strings, "2")
	assert.False(s.Remove("1"))
	assert.True(s.Remove("2"))
	assert.Len(s.Strings, 0)
}

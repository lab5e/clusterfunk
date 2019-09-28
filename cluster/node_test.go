package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNode(t *testing.T) {
	assert := require.New(t)

	n := NewNode("A", Follower)
	assert.Equal("A", n.ID)
	assert.Equal(Follower, n.Role)
	assert.Equal(0, len(n.Tags))

	tags := map[string]string{
		"one": "two",
	}
	n.SetTags(tags)

	delete(tags, "one")

	assert.Equal(1, len(n.Tags))
	assert.Equal("two", n.Tags["one"])
}

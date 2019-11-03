package funk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAckCollectionHappyPath(t *testing.T) {
	assert := require.New(t)

	c := newAckCollection()
	assert.NotNil(c)

	c.StartAck([]string{"A", "B", "C"}, 100*time.Millisecond)
	defer c.Done()

	c.Ack("A")
	c.Ack("B")
	c.Ack("C")

	select {
	case <-c.Completed():
		// success
	case <-c.MissingAck():
		assert.Fail("Should not miss ack")
	case <-time.After(500 * time.Millisecond):
		assert.Fail("One of the channels should contain a response")
	}
}

func TestAckCollectionMissingAck(t *testing.T) {
	assert := require.New(t)

	c := newAckCollection()
	assert.NotNil(c)

	c.StartAck([]string{"A", "B", "C"}, 100*time.Millisecond)
	defer c.Done()

	c.Ack("A")
	c.Ack("C")

	select {
	case <-c.Completed():
		assert.Fail("Should not be complete")
	case <-c.MissingAck():
		// success
	case <-time.After(500 * time.Millisecond):
		assert.Fail("One of the channels should contain a response")
	}
}

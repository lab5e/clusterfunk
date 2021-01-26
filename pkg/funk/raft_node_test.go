package funk

import (
	"testing"

	"github.com/lab5e/gotoolbox/netutils"
	"github.com/stretchr/testify/require"
)

func TestRaftCluster(t *testing.T) {
	assert := require.New(t)

	dummyLog := LogMessage{
		MessageType: ProposedShardMap,
		AckEndpoint: "foofoo",
		Data:        []byte{1, 2, 3},
	}
	dummyLogBuffer, _ := dummyLog.MarshalBinary()

	id1 := "node1"
	id2 := "node2"
	id3 := "node3"
	params1 := RaftParameters{Bootstrap: true, Endpoint: netutils.RandomLocalEndpoint()}
	params2 := RaftParameters{Bootstrap: false, Endpoint: netutils.RandomLocalEndpoint()}
	params3 := RaftParameters{Bootstrap: false, Endpoint: netutils.RandomLocalEndpoint()}

	// Make a three-node cluster
	node1 := NewRaftNode()

	evts1 := node1.Events()

	bs, err := node1.Start(id1, params1)
	assert.NoError(err, "Start should be successful")
	assert.True(bs)

	waitForEvent := func(ev RaftEventType, ch <-chan RaftEventType) {
		lastEvent := RaftEventType(-1)
		for lastEvent != ev {
			lastEvent = <-ch
		}
	}

	waitForEvent(RaftBecameLeader, evts1)
	assert.Equal(node1.LocalNodeID(), id1)

	assert.NoError(node1.Stop(true), "Did not expect error when stopping")

	assert.Error(node1.Stop(false), "Expected error when stopping a 2nd time")

	bs, err = node1.Start(id1, params1)
	assert.True(bs, "Bootstrap 2nd time succeeds")
	assert.NoError(err, "2nd start should be success")
	waitForEvent(RaftBecameLeader, evts1)

	node2 := NewRaftNode()

	bs, err = node2.Start(id2, params2)
	assert.False(bs, "2nd node should not bootstrap")
	assert.NoError(err, "2nd node should launch")

	assert.NoError(node1.AddClusterNode(id2, params2.Endpoint), "Node 2 should join successfully")

	evts2 := node2.Events()
	waitForEvent(RaftBecameFollower, evts2)
	assert.Equal(node2.LocalNodeID(), id2)
	assert.Equal(node2.Endpoint(), params2.Endpoint)

	node3 := NewRaftNode()
	evts3 := node3.Events()
	bs, err = node3.Start(id3, params3)
	assert.False(bs, "3rd node should not bootstrap")
	assert.NoError(err)
	assert.NoError(node1.AddClusterNode(id3, params3.Endpoint))

	waitForEvent(RaftBecameFollower, evts3)

	assert.True(node1.Leader())
	assert.False(node2.Leader())
	assert.False(node3.Leader())

	_, err = node3.AppendLogEntry(dummyLogBuffer)
	assert.Error(err, "Should get error when appending log entry and isn't leader")

	index, err := node1.AppendLogEntry(dummyLogBuffer)

	assert.NoError(err, "No error when appending log on leader")

	waitForEvent(RaftReceivedLog, evts1)
	waitForEvent(RaftReceivedLog, evts2)
	waitForEvent(RaftReceivedLog, evts3)

	assert.Equal(index, node2.LastLogIndex())
	assert.Equal(index, node3.LastLogIndex())

	msgs := node3.GetLogMessages(0)
	assert.Len(msgs, 1)

	// Removing and adding node should work
	assert.NoError(node1.RemoveClusterNode(id2, params2.Endpoint))
	assert.NoError(node1.AddClusterNode(id2, params2.Endpoint))

	assert.NoError(node1.Stop(true))

	waitForEvent(RaftLeaderLost, evts2)
	waitForEvent(RaftLeaderLost, evts3)

	assert.NoError(node3.Stop(true))
	assert.NoError(node2.Stop(true))

}

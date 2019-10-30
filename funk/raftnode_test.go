package funk

import (
	"sync"
	"testing"

	"github.com/stalehd/clusterfunk/toolbox"
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
	params1 := RaftParameters{Bootstrap: true, DiskStore: false, RaftEndpoint: toolbox.RandomLocalEndpoint()}
	params2 := RaftParameters{Bootstrap: false, DiskStore: false, RaftEndpoint: toolbox.RandomLocalEndpoint()}
	params3 := RaftParameters{Bootstrap: false, DiskStore: false, RaftEndpoint: toolbox.RandomLocalEndpoint()}

	// Make a three-node cluster
	node1 := NewRaftNode()

	evts1 := node1.Events()

	assert.NoError(node1.Start(id1, false, params1), "Start should be successful")

	waitForEvent := func(ev RaftEventType, ch <-chan RaftEventType) {
		lastEvent := RaftEventType(-1)
		for lastEvent != ev {
			lastEvent = <-ch
		}
	}

	waitForEvent(RaftBecameLeader, evts1)
	assert.Equal(node1.LocalNodeID(), id1)

	assert.NoError(node1.Stop(), "Did not expect error when stopping")

	assert.Error(node1.Stop(), "Expected error when stopping a 2nd time")

	assert.NoError(node1.Start(id1, false, params1), "2nd start should be success")
	waitForEvent(RaftBecameLeader, evts1)

	node2 := NewRaftNode()
	//evts2 := node2.Events()
	assert.NoError(node2.Start(id2, false, params2), "2nd node should launch")

	assert.NoError(node1.AddClusterNode(id2, params2.RaftEndpoint), "Node 2 should join successfully")

	evts2 := node2.Events()
	waitForEvent(RaftBecameFollower, evts2)
	assert.Equal(node2.LocalNodeID(), id2)
	assert.Equal(node2.Endpoint(), params2.RaftEndpoint)

	node3 := NewRaftNode()
	evts3 := node3.Events()
	assert.NoError(node3.Start(id3, false, params3))
	assert.NoError(node1.AddClusterNode(id3, params3.RaftEndpoint))

	waitForEvent(RaftBecameFollower, evts3)

	assert.True(node1.Leader())
	assert.False(node2.Leader())
	assert.False(node3.Leader())

	_, err := node3.AppendLogEntry(dummyLogBuffer)
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
	assert.NoError(node1.RemoveClusterNode(id2, params2.RaftEndpoint))
	assert.NoError(node1.AddClusterNode(id2, params2.RaftEndpoint))

	assert.NoError(node1.Stop())

	waitForEvent(RaftLeaderLost, evts2)
	waitForEvent(RaftLeaderLost, evts3)

	assert.NoError(node3.Stop())
	assert.NoError(node2.Stop())

}

func TestRaftEventCoalesce(t *testing.T) {
	assert := require.New(t)

	rn := NewRaftNode()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	rn.sendInternalEvent(RaftLeaderLost)

	go rn.coalescingEvents()

	for _, ev := range []RaftEventType{
		RaftLeaderLost,
		RaftBecameLeader,
		RaftClusterSizeChanged,
		RaftClusterSizeChanged,
	} {
		rn.sendInternalEvent(ev)
	}

	expectedEvents := []RaftEventType{
		RaftLeaderLost,
		RaftBecameLeader,
		RaftClusterSizeChanged,
	}
	for ev := range rn.Events() {
		assert.Equal(expectedEvents[0], ev, "Expecting %s got %s", expectedEvents[0], ev)
		expectedEvents = expectedEvents[1:]
		if len(expectedEvents) == 0 {
			break
		}
	}

	for _, ev := range []RaftEventType{
		RaftLeaderLost,
		RaftLeaderLost,
		RaftBecameFollower,
		RaftClusterSizeChanged,
		RaftClusterSizeChanged,
	} {
		rn.sendInternalEvent(ev)
	}

	expectedEvents = []RaftEventType{
		RaftLeaderLost,
		RaftBecameFollower,
		RaftClusterSizeChanged,
	}
	for ev := range rn.Events() {
		assert.Equal(expectedEvents[0], ev)
		expectedEvents = expectedEvents[1:]
		if len(expectedEvents) == 0 {
			break
		}
	}

	for _, ev := range []RaftEventType{
		RaftReceivedLog,
		RaftReceivedLog,
		RaftReceivedLog,
		RaftReceivedLog,
	} {
		rn.sendInternalEvent(ev)
	}

	expectedEvents = []RaftEventType{
		RaftReceivedLog,
	}
	for ev := range rn.Events() {
		assert.Equal(expectedEvents[0], ev)
		expectedEvents = expectedEvents[1:]
		if len(expectedEvents) == 0 {
			break
		}
	}
}

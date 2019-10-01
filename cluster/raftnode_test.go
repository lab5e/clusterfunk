package cluster

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

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

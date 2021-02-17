package funk

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNameMatcher(t *testing.T) {
	assert := require.New(t)

	assert.False(endpointNameMatches("ep.name", "ep."))
	assert.False(endpointNameMatches("ep.name", "ep.n"))
	assert.True(endpointNameMatches("ep.name", "ep.name"))
	assert.False(endpointNameMatches("ep.name", "na"))
	assert.True(endpointNameMatches("ep.name", "name"))
	assert.False(endpointNameMatches("ep.name", ""))

	assert.False(endpointNameMatches("ep.name", "namename"))
	assert.False(endpointNameMatches("ep.name", "ep.nom"))
	assert.False(endpointNameMatches("ep.name", "nom"))
}

func TestObserverInputEvents(t *testing.T) {
	assert := require.New(t)

	ch := make(chan NodeEvent)
	em := NewEndpointObserver("local", ch, nil)
	defer em.Shutdown()

	var events []NodeEvent
	// Generate 10 nodes with 10 endpoints via events
	for i := 0; i < 10; i++ {
		ev := NodeEvent{
			Event: SerfNodeJoined,
			Node: SerfMember{
				NodeID: fmt.Sprintf("id%d", i),
				State:  SerfAlive,
				Tags: map[string]string{
					"ep.test": "listenaddress",
				},
			},
		}
		events = append(events, ev)
		ch <- ev
	}
	// Dump the last event into the channel to ensure everything is read
	// (the channel is unbuffered)
	ch <- events[0]
	endpoints := em.Endpoints()
	assert.Len(endpoints, 10)

	endpoints = em.Find("ep.test")
	assert.Len(endpoints, 10)

	ep, err := em.FindFirst("ep.test")
	assert.NoError(err)
	assert.Equal("listenaddress", ep.ListenAddress)

	// Generate 10 updates, add a new endpoint and update the existing
	for i, ev := range events {
		ev.Event = SerfNodeUpdated
		ev.Node.Tags["ep.test"] = "listen2"
		ev.Node.Tags["ep.test2"] = "listen3"
		events[i] = ev
		ch <- ev
	}
	ch <- events[2]

	endpoints = em.Endpoints()
	assert.Len(endpoints, 20)

	endpoints = em.Find("ep.test2")
	assert.Len(endpoints, 10)

	endpoints = em.Find("ep.test")
	assert.Len(endpoints, 10)

	endpoints = em.Find("ep.unknown")
	assert.Len(endpoints, 0)

	_, err = em.FindFirst("ep.test2")
	assert.NoError(err)
	_, err = em.FindFirst("ep.unknown")
	assert.Error(err)

	// Send updates with the node state set to SerfFailed. The endpoints should
	// be removed
	for _, ev := range events {
		ev.Event = SerfNodeUpdated
		ev.Node.State = SerfFailed
		ch <- ev
	}

	// Another round, update
	for _, ev := range events {
		ev.Event = SerfNodeUpdated
		ch <- ev
	}
	ch <- events[1]

	endpoints = em.Endpoints()
	assert.Len(endpoints, 20)

	for i, ev := range events {
		ev.Event = SerfNodeLeft
		events[i] = ev
		ch <- ev
	}
	ch <- events[1]
	endpoints = em.Endpoints()
	assert.Len(endpoints, 0)

}

// Ensure observers are updated accordingly
func TestObserverOutputEvents(t *testing.T) {
	assert := require.New(t)

	ch := make(chan NodeEvent)
	em := NewEndpointObserver("local", ch, nil)
	defer em.Shutdown()

	activeCount := new(int32)
	inactiveCount := new(int32)
	atomic.StoreInt32(activeCount, 0)
	atomic.StoreInt32(inactiveCount, 0)

	observerFunc := func(obsChan <-chan Endpoint) {
		for ep := range obsChan {
			if ep.Active {
				atomic.AddInt32(activeCount, 1)
				continue
			}
			atomic.AddInt32(inactiveCount, 1)
		}
	}
	obsCh := em.Observe()
	go observerFunc(obsCh)
	go observerFunc(em.Observe())

	var events []NodeEvent
	// Generate 10 nodes with 10 endpoints via events
	for i := 0; i < 10; i++ {
		ev := NodeEvent{
			Event: SerfNodeJoined,
			Node: SerfMember{
				NodeID: fmt.Sprintf("id%d", i),
				State:  SerfAlive,
				Tags: map[string]string{
					"ep.test": "listenaddress",
				},
			},
		}
		events = append(events, ev)
		ch <- ev
	}

	// Busy wait until active is 10 since we don't know when
	// the event will be processed. There will be twice as
	// many events going out since we have two observers.
	n := 0
	for {
		if atomic.LoadInt32(activeCount) == 20 {
			break
		}
		time.Sleep(1 * time.Millisecond)
		n++
		assert.Less(n, 1000)
	}

	// Generate 10 remove events and repeat busy wait
	for _, ev := range events {
		ev.Event = SerfNodeLeft
		ch <- ev
	}

	n = 0
	for {
		if atomic.LoadInt32(inactiveCount) == 20 {
			break
		}
		time.Sleep(1 * time.Millisecond)
		n++
		assert.Less(n, 1000)
	}

	em.Unobserve(obsCh)
}

// Fairly simple test, just ensure existing endpoints are picked up
func TestExistingEndpointObserver(t *testing.T) {
	assert := require.New(t)

	ch := make(chan NodeEvent)
	em := NewEndpointObserver("local", ch, []Endpoint{
		{
			Active:        true,
			Cluster:       true,
			Name:          "ep.existing",
			ListenAddress: "local",
		},
	})
	defer em.Shutdown()

	assert.Len(em.Endpoints(), 1)
	ep, err := em.FindFirst("ep.existing")
	assert.NoError(err)
	assert.Equal("local", ep.ListenAddress)
}

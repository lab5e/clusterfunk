package funk

import (
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestNameMatcher(t *testing.T) {
	assert := require.New(t)

	assert.True(endpointNameMatches("ep.name", "ep."))
	assert.True(endpointNameMatches("ep.name", "ep.n"))
	assert.True(endpointNameMatches("ep.name", "ep.name"))
	assert.True(endpointNameMatches("ep.name", "na"))
	assert.True(endpointNameMatches("ep.name", "name"))
	assert.True(endpointNameMatches("ep.name", ""))

	assert.False(endpointNameMatches("ep.name", "namename"))
	assert.False(endpointNameMatches("ep.name", "ep.nom"))
	assert.False(endpointNameMatches("ep.name", "nom"))
}

// Launch a local Serf node, register a few endpoints and ensure they're registered
func TestObserver(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	assert := require.New(t)

	ch := make(chan NodeEvent)
	em := NewEndpointObserver("local", ch, nil)
	defer em.Shutdown()

	// Generate a few add and remove events. The actual implementation would
	// use events from a SerfNode instance here.
	ch <- NodeEvent{
		Event: SerfNodeJoined,
		Node: SerfMember{
			NodeID: "local",
			State:  "active",
			Tags: map[string]string{
				"ep.no1": "localhost:1",
				"ep.no2": "localhost:2",
				"ep.no3": "localhost:3",
			},
		}}
	ch <- NodeEvent{
		Event: SerfNodeJoined,
		Node: SerfMember{
			NodeID: "remote",
			State:  "active",
			Tags: map[string]string{
				"ep.no1": "remote:1",
				"ep.no2": "remote:2",
				"ep.no3": "remote:3",
			},
		}}
	ep, err := em.FindFirst("ep.no1")
	assert.NoError(err)
	assert.Equal(ep.Name, "ep.no1")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		obs := em.Observe()
		for ev := range obs {
			logrus.Debugf("Got event: %+v", ev)
			if ev.ListenAddress == "last:1" {
				wg.Done()
			}
		}
	}()

	ch <- NodeEvent{
		Event: SerfNodeUpdated,
		Node: SerfMember{
			NodeID: "remote2",
			State:  "active",
			Tags: map[string]string{
				"ep.no4":  "remote2:4",
				"ep.raft": "remote2:2",
			},
		}}

	ch <- NodeEvent{
		Event: SerfNodeLeft,
		Node: SerfMember{
			NodeID: "remote",
			State:  "inactive",
			Tags: map[string]string{
				"ep.no1": "remote:1",
			},
		}}

	ch <- NodeEvent{
		Event: SerfNodeLeft,
		Node: SerfMember{
			NodeID: "last",
			State:  "inactive",
			Tags: map[string]string{
				"ep.no999": "last:1",
			},
		}}

	wg.Wait()

	eps := em.Endpoints()
	assert.NotNil(eps)
	assert.Len(eps, 7)

	ep, err = em.FindFirst("ep.raft")
	assert.NoError(err)
	assert.Equal("ep.raft", ep.Name)
	assert.Equal("remote2:2", ep.ListenAddress)
	assert.True(ep.Cluster)

	eps = em.Find("ep.no1")
	assert.Len(eps, 1)

	// Race condition!, who's there? Knock knock!
	ready := &sync.WaitGroup{}
	ready.Add(1)
	wg.Add(1)
	go func() {
		logrus.Debug("Wait")
		ready.Done()
		em.WaitForEndpoints()
		logrus.Debug("Done waiting")
		wg.Done()
	}()
	ready.Wait()
	ch <- NodeEvent{
		Event: SerfNodeUpdated,
		Node: SerfMember{
			NodeID: "up",
			State:  "inactive",
			Tags: map[string]string{
				"ep.no99": "up:1",
			},
		}}
	wg.Wait()
}

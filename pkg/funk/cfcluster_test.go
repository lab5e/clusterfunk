package funk

import (
	"context"
	"testing"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/netutils"
	"github.com/stretchr/testify/require"
)

func waitForClusterEvent(ev <-chan Event, state NodeState) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	for {
		select {
		case v := <-ev:
			if v.State == state {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

func TestCfCluster(t *testing.T) {
	assert := require.New(t)
	sm1 := sharding.NewShardMap()
	assert.NoError(sm1.Init(1000))

	params1 := Parameters{
		Raft: RaftParameters{
			RaftEndpoint: netutils.RandomLocalEndpoint(),
			DiskStore:    false,
			Bootstrap:    true,
		},
		Serf: SerfParameters{
			Endpoint:    netutils.RandomLocalEndpoint(),
			JoinAddress: []string{},
		},
		AutoJoin:   true,
		Name:       "testCluster",
		ZeroConf:   true,
		NodeID:     toolbox.RandomID(),
		AckTimeout: 1 * time.Second,
	}
	params1.Final()
	clusterNode1, err := NewCluster(params1, sm1)

	assert.NotNil(clusterNode1)
	assert.NoError(err)

	assert.NoError(clusterNode1.Start())

	assert.True(waitForClusterEvent(clusterNode1.Events(), Operational), "Node 1 should be operational")

	// Shard manager should contain a single node
	assert.Len(sm1.NodeList(), 1)
	assert.Equal(sm1.NodeList()[0], params1.NodeID)
	defer clusterNode1.Stop()
}

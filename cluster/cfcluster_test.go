package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/toolbox"
	"github.com/stretchr/testify/require"
)

func waitForClusterEvent(ev <-chan Event, state NodeState) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
	sm1 := sharding.NewShardManager()
	sm1.Init(10000, nil)

	params1 := Parameters{
		Raft: RaftParameters{
			RaftEndpoint: randomEndpoint(),
			DiskStore:    false,
			Bootstrap:    true,
		},
		Serf: SerfParameters{
			Endpoint:    randomEndpoint(),
			JoinAddress: "",
		},
		AutoJoin:    true,
		ClusterName: "testCluster",
		ZeroConf:    true,
		NodeID:      toolbox.RandomID(),
	}
	params1.Final()
	clusterNode1 := NewCluster(params1, sm1)

	assert.NotNil(clusterNode1)

	assert.NoError(clusterNode1.Start())

	assert.True(waitForClusterEvent(clusterNode1.Events(), Operational), "Node 1 should be operational")

	// Shard manager should contain a single node
	assert.Len(sm1.NodeList(), 1)
	assert.Equal(sm1.NodeList()[0], params1.NodeID)
	defer clusterNode1.Stop()
}

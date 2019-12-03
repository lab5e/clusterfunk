package funk
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"context"
	"testing"
	"time"

	"github.com/stalehd/clusterfunk/pkg/funk/sharding"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
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
	assert.NoError(sm1.Init(10000, nil))

	params1 := Parameters{
		Raft: RaftParameters{
			RaftEndpoint: toolbox.RandomLocalEndpoint(),
			DiskStore:    false,
			Bootstrap:    true,
		},
		Serf: SerfParameters{
			Endpoint:    toolbox.RandomLocalEndpoint(),
			JoinAddress: "",
		},
		AutoJoin:         true,
		Name:             "testCluster",
		ZeroConf:         true,
		NodeID:           toolbox.RandomID(),
		LivenessInterval: 150 * time.Millisecond,
		AckTimeout:       1 * time.Second,
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

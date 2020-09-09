package clientfunk

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
	"testing"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestClusterResolverBuilder(t *testing.T) {
	assert := require.New(t)

	delete(resolverBuilder.resolverEndpoints, "ep.test")
	resolverBuilder.addEndpoint(funk.Endpoint{
		Name:          "ep.test",
		ListenAddress: "example.com:1234"})

	conn, err := grpc.Dial("cluster:///ep.test", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(GRPCServiceConfig))
	assert.NoError(err)
	assert.NotNil(conn)
	defer conn.Close()

	_, err = grpc.Dial("cluster:///xep.test", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(GRPCServiceConfig))
	assert.Error(err)
}

// Do a white box test of the resolver just to ensure basic coverage.
func TestResolverBuilder(t *testing.T) {
	assert := require.New(t)
	for k := range resolverBuilder.resolverEndpoints {
		delete(resolverBuilder.resolverEndpoints, k)
	}
	resolverBuilder.updateEndpoints([]funk.Endpoint{
		funk.Endpoint{Name: "ep.1", ListenAddress: "example.com:1"},
		funk.Endpoint{Name: "ep.2", ListenAddress: "example.com:1"},
		funk.Endpoint{Name: "ep.3", ListenAddress: "example.com:1"},
	})
	resolverBuilder.addEndpoint(funk.Endpoint{
		Name:          "ep.5",
		ListenAddress: "example.com:2222",
	})
	resolverBuilder.removeEndpoint(funk.Endpoint{
		Name:          "ep.5",
		ListenAddress: "something.example.com:1234",
	})
	assert.Len(resolverBuilder.resolverEndpoints, 4)
	resolverBuilder.removeEndpoint(funk.Endpoint{
		Name:          "ep.1",
		ListenAddress: "example.com:1",
	})
	resolverBuilder.removeEndpoint(funk.Endpoint{
		Name:          "ep.2",
		ListenAddress: "example.com:1",
	})
	resolverBuilder.removeEndpoint(funk.Endpoint{
		Name:          "ep.3",
		ListenAddress: "example.com:1",
	})
	assert.Len(resolverBuilder.resolverEndpoints, 1)
	resolverBuilder.removeEndpoint(funk.Endpoint{
		Name:          "ep.5",
		ListenAddress: "example.com:2222",
	})
	assert.Len(resolverBuilder.resolverEndpoints, 0)

	eventChan := make(chan funk.NodeEvent)
	em := funk.NewEndpointObserver("local", eventChan, nil)
	resolverBuilder.registerObserver(em)
	resolverBuilder.registerObserver(em)

	defer em.Shutdown()
	eventChan <- funk.NodeEvent{
		Event: funk.SerfNodeJoined,
		Node: funk.SerfMember{
			NodeID: "remote",
			State:  funk.SerfAlive,
			Tags: map[string]string{
				"ep.nn": "example.com:99",
			},
		},
	}
	eventChan <- funk.NodeEvent{
		Event: funk.SerfNodeLeft,
		Node: funk.SerfMember{
			NodeID: "remote",
			State:  funk.SerfAlive,
			Tags: map[string]string{
				"ep.nn": "example.com:99",
			},
		},
	}

	time.Sleep(10 * time.Millisecond)
}

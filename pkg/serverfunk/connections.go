package serverfunk

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
	"errors"
	"sync"

	"github.com/lab5e/clusterfunk/pkg/funk"

	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// ProxyConnections manages grpc.ClientConn connections to proxies.
type ProxyConnections struct {
	Shards           sharding.ShardMap
	Cluster          funk.Cluster
	EndpointName     string
	mutex            *sync.Mutex
	grpcClients      map[string]*grpc.ClientConn
	operationalMutex *sync.RWMutex
}

// NewProxyConnections creates a new GRPCClientProxy
func NewProxyConnections(endpointName string, shards sharding.ShardMap, cluster funk.Cluster) *ProxyConnections {
	ret := &ProxyConnections{
		Shards:           shards,
		Cluster:          cluster,
		EndpointName:     endpointName,
		mutex:            &sync.Mutex{},
		grpcClients:      make(map[string]*grpc.ClientConn),
		operationalMutex: &sync.RWMutex{},
	}
	go ret.clusterEventListener(cluster.Events())
	return ret
}

func (p *ProxyConnections) clusterEventListener(evts <-chan funk.Event) {
	once := &sync.Once{}
	for ev := range evts {
		if ev.State == funk.Operational {
			p.operationalMutex.Unlock()
			once = &sync.Once{}
		} else {
			once.Do(func() {
				p.operationalMutex.Lock()
			})
		}
	}
}

// Options returns the grpc.DialOption to use when creating new connections
func (p *ProxyConnections) Options() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithInsecure(),
	}
}

// GetConnection returns a gRPC connection and node ID to the service handling the shard (ID). If the shard is handled locally it will return a nil connection
func (p *ProxyConnections) GetConnection(shard int) (*grpc.ClientConn, string, error) {
	p.operationalMutex.RLock()
	defer p.operationalMutex.RUnlock()
	nodeID := p.Shards.MapToNode(shard).NodeID()
	if nodeID == p.Cluster.NodeID() {
		return nil, nodeID, nil
	}
	endpoint := p.Cluster.GetEndpoint(nodeID, p.EndpointName)
	if endpoint == "" {
		logrus.WithFields(logrus.Fields{
			"nodeid":   nodeID,
			"endpoint": p.EndpointName}).Error("Can't find endpoint for node")
		return nil, nodeID, errors.New("can't map request to node")
	}

	// Look up the client in the map
	p.mutex.Lock()
	defer p.mutex.Unlock()
	conn, ok := p.grpcClients[endpoint]
	if !ok {
		// Create a new client
		opts := p.Options()
		var err error
		conn, err = grpc.Dial(endpoint, opts...)
		if err != nil {
			return nil, nodeID, err
		}
		p.grpcClients[endpoint] = conn
	}
	return conn, nodeID, nil
}

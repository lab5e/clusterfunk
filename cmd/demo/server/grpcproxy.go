package main

import (
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/cluster"
	"github.com/stalehd/clusterfunk/cluster/sharding"
	"google.golang.org/grpc"
)

// ClientFactoryFunc is a factory function for gRPC clients
type ClientFactoryFunc func(*grpc.ClientConn) interface{}

// GRPCClientProxy can automatically resolve the client proxying
type GRPCClientProxy struct {
	Shards           sharding.ShardManager
	Cluster          cluster.Cluster
	EndpointName     string
	mutex            *sync.Mutex
	grpcClients      map[string]interface{}
	clientFactory    ClientFactoryFunc
	operationalMutex *sync.RWMutex
}

// NewGRPCClientProxy creates a new GRPCClientProxy
func NewGRPCClientProxy(endpointName string, clientFactory ClientFactoryFunc, shards sharding.ShardManager, c cluster.Cluster) *GRPCClientProxy {
	ret := &GRPCClientProxy{
		Shards:           shards,
		Cluster:          c,
		EndpointName:     endpointName,
		mutex:            &sync.Mutex{},
		grpcClients:      make(map[string]interface{}),
		clientFactory:    clientFactory,
		operationalMutex: &sync.RWMutex{},
	}
	go ret.clusterEventListener(c.Events())
	return ret
}

func (p *GRPCClientProxy) clusterEventListener(evts <-chan cluster.Event) {
	once := &sync.Once{}
	for ev := range evts {
		if ev.State == cluster.Operational {
			log.Warn("Unlocking operational mutex since we're operational")
			p.operationalMutex.Unlock()
			once = &sync.Once{}
		} else {
			once.Do(func() {
				log.Warnf("Locking operational mutex since state is %s", ev.State)
				p.operationalMutex.Lock()
			})
		}
	}
}

// GetProxyClient returns the proxy client for the given shard. If there's an
// error the client will be nil and the error is set. If  both fields are nil
// the local node is the one serving the request
func (p *GRPCClientProxy) GetProxyClient(shard int) (interface{}, error) {
	p.operationalMutex.RLock()
	defer p.operationalMutex.RUnlock()
	nodeID := p.Shards.MapToNode(shard).NodeID()
	if nodeID == p.Cluster.NodeID() {
		return nil, nil
	}
	endpoint := p.Cluster.GetEndpoint(nodeID, p.EndpointName)
	if endpoint == "" {
		log.WithFields(log.Fields{
			"nodeid":   nodeID,
			"endpoint": p.EndpointName}).Error("Can't find endpoint for node")
		return nil, errors.New("can't map request to node")
	}

	// Look up the client in the map
	p.mutex.Lock()
	defer p.mutex.Unlock()
	ret, ok := p.grpcClients[endpoint]
	if !ok {
		// Create a new client
		opts := []grpc.DialOption{grpc.WithInsecure()}
		conn, err := grpc.Dial(endpoint, opts...)
		if err != nil {
			return nil, err
		}
		ret = p.clientFactory(conn)
		p.grpcClients[endpoint] = ret
	}
	return ret, nil
}

package main

import (
	"context"
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/cluster"
	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/cmd/demo"
	"google.golang.org/grpc"
)

type liffProxy struct {
	localLiff    demo.DemoServiceServer
	shards       sharding.ShardManager
	shardFromID  sharding.ShardFunc
	demoCluster  cluster.Cluster
	endpointName string
	grpcClients  map[string]demo.DemoServiceClient
	mutex        *sync.Mutex
	clientProxy  *GRPCClientProxy
}

func newLiffProxy(localLiff demo.DemoServiceServer, shardMap sharding.ShardManager, c cluster.Cluster, endpointName string) demo.DemoServiceServer {
	makeDemoClient := func(conn *grpc.ClientConn) interface{} {
		return demo.NewDemoServiceClient(conn)
	}
	return &liffProxy{
		localLiff:    localLiff,
		shards:       shardMap,
		shardFromID:  sharding.NewIntSharder(int64(len(shardMap.Shards()))),
		clientProxy:  NewGRPCClientProxy(endpointName, makeDemoClient, shardMap, c),
		demoCluster:  c,
		endpointName: endpointName,
		grpcClients:  make(map[string]demo.DemoServiceClient),
		mutex:        &sync.Mutex{},
	}
}

// getProxyClient returns the proxy client for the given shard. If there's an
// error the client will be nil and the error is set. If  both fields are nil
// the local node is the one serving the request
func (lp *liffProxy) getProxyClient(shard int) (interface{}, error) {
	// TODO: Check state of cluster. If it's operational
	nodeID := lp.shards.MapToNode(shard).NodeID()
	if nodeID == lp.demoCluster.NodeID() {
		return nil, nil
	}
	endpoint := lp.demoCluster.GetEndpoint(nodeID, lp.endpointName)
	if endpoint == "" {
		log.WithFields(log.Fields{
			"nodeid":   nodeID,
			"endpoint": lp.endpointName}).Error("Can't find endpoint for node")
		return nil, errors.New("can't map request to node")
	}

	// Look up the client in the map
	lp.mutex.Lock()
	defer lp.mutex.Unlock()
	ret, ok := lp.grpcClients[endpoint]
	if !ok {
		// Create a new client
		opts := []grpc.DialOption{grpc.WithInsecure()}
		conn, err := grpc.Dial(endpoint, opts...)
		if err != nil {
			return nil, err
		}
		ret = demo.NewDemoServiceClient(conn)
		lp.grpcClients[endpoint] = ret
	}
	return ret, nil
}

func (lp *liffProxy) Liff(ctx context.Context, req *demo.LiffRequest) (*demo.LiffResponse, error) {
	client, err := lp.clientProxy.GetProxyClient(lp.shardFromID(req.ID))
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client.(demo.DemoServiceClient).Liff(ctx, req)
	}
	return lp.localLiff.Liff(ctx, req)
}

func (lp *liffProxy) MakeKeyPair(ctx context.Context, req *demo.KeyPairRequest) (*demo.KeyPairResponse, error) {
	client, err := lp.getProxyClient(lp.shardFromID(req.ID))
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client.(demo.DemoServiceClient).MakeKeyPair(ctx, req)
	}
	return lp.localLiff.MakeKeyPair(ctx, req)
}

func (lp *liffProxy) Slow(ctx context.Context, req *demo.SlowRequest) (*demo.SlowResponse, error) {
	client, err := lp.getProxyClient(lp.shardFromID(req.ID))
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client.(demo.DemoServiceClient).Slow(ctx, req)
	}
	return lp.localLiff.Slow(ctx, req)
}

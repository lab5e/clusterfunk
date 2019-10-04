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
}

func newLiffProxy(localLiff demo.DemoServiceServer, shardMap sharding.ShardManager, c cluster.Cluster, endpointName string) demo.DemoServiceServer {
	return &liffProxy{
		localLiff:    localLiff,
		shards:       shardMap,
		shardFromID:  sharding.NewIntSharder(int64(len(shardMap.Shards()))),
		demoCluster:  c,
		endpointName: endpointName,
		grpcClients:  make(map[string]demo.DemoServiceClient),
		mutex:        &sync.Mutex{},
	}
}

func (lp *liffProxy) getClientForEndpoint(endpoint string) (demo.DemoServiceClient, error) {
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
	nodeID := lp.shards.MapToNode(lp.shardFromID(req.ID)).NodeID()
	if nodeID == lp.demoCluster.NodeID() {
		//log.WithField("id", req.ID).WithField("nodeid", nodeID).Info("Handling Liff locally")
		return lp.localLiff.Liff(ctx, req)
	}
	endpoint := lp.demoCluster.GetEndpoint(nodeID, lp.endpointName)
	if endpoint == "" {
		log.WithFields(log.Fields{
			"nodeid":   nodeID,
			"endpoint": lp.endpointName}).Error("Can't find endpoint for node")
		return nil, errors.New("can't map request to node")
	}

	client, err := lp.getClientForEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	//	log.WithField("id", req.ID).WithField("nodeid", nodeID).Info("Proxying Liff request")
	return client.Liff(ctx, req)
}

func (lp *liffProxy) MakeKeyPair(ctx context.Context, req *demo.KeyPairRequest) (*demo.KeyPairResponse, error) {
	return nil, errors.New("not implemented")
}

func (lp *liffProxy) Slow(ctx context.Context, req *demo.SlowRequest) (*demo.SlowResponse, error) {
	return nil, errors.New("not implemented")
}

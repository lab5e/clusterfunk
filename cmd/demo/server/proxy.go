package main

import (
	"context"

	"github.com/stalehd/clusterfunk/clientfunk"

	"github.com/stalehd/clusterfunk/funk"

	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/funk/sharding"
	"google.golang.org/grpc"
)

type liffProxy struct {
	localLiff   demo.DemoServiceServer
	shardFromID sharding.ShardFunc
	clientProxy *clientfunk.GRPCClientProxy
}

func newLiffProxy(localLiff demo.DemoServiceServer, shardMap sharding.ShardManager, c funk.Cluster, endpointName string) demo.DemoServiceServer {
	makeDemoClient := func(conn *grpc.ClientConn) interface{} {
		return demo.NewDemoServiceClient(conn)
	}
	return &liffProxy{
		localLiff:   localLiff,
		shardFromID: sharding.NewIntSharder(int64(len(shardMap.Shards()))),
		clientProxy: clientfunk.NewGRPCClientProxy(endpointName, makeDemoClient, shardMap, c),
	}
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
	client, err := lp.clientProxy.GetProxyClient(lp.shardFromID(req.ID))
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client.(demo.DemoServiceClient).MakeKeyPair(ctx, req)
	}
	return lp.localLiff.MakeKeyPair(ctx, req)
}

func (lp *liffProxy) Slow(ctx context.Context, req *demo.SlowRequest) (*demo.SlowResponse, error) {
	client, err := lp.clientProxy.GetProxyClient(lp.shardFromID(req.ID))
	if err != nil {
		return nil, err
	}
	if client != nil {
		return client.(demo.DemoServiceClient).Slow(ctx, req)
	}
	return lp.localLiff.Slow(ctx, req)
}

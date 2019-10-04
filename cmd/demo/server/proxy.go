package main

import (
	"context"

	"github.com/stalehd/clusterfunk/clientfunk"

	"github.com/stalehd/clusterfunk/funk"

	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/funk/sharding"
	"google.golang.org/grpc"
)

// This is a proxying server implementation that uses the GRPCClientProxy
// type in the clientfunk package. It demonstrates how to proxy requests
// sideways in the cluster. There's nothing stopping you from putting the
// client code in the same server but the code is separated out to make the
// code a bit easier to read.

type liffProxy struct {
	localLiff   demo.DemoServiceServer
	shardFromID sharding.ShardFunc
	clientProxy *clientfunk.GRPCClientProxy
}

func newLiffProxy(
	localLiff demo.DemoServiceServer, // The local implementation of the server
	shardMap sharding.ShardManager, // The shard map to use
	c funk.Cluster, // The cluster that manages the shard map
	endpointName string, // The name of the endpoint we'll proxy to
) demo.DemoServiceServer {

	// This is the factory function for demo client services. It will be used
	// to create proxies from gRPC client connections.
	makeDemoClient := func(conn *grpc.ClientConn) interface{} {
		return demo.NewDemoServiceClient(conn)
	}

	// This is the shard function. It's just a simple function that converts
	// the identifier into a shard in the range [0..max shards].
	shardFunction := sharding.NewIntSharder(int64(len(shardMap.Shards())))

	// This is the proxying code. It will block requests while the cluster is
	// busy resharding itself, f.e. when a new node is added or a node has died.
	proxy := clientfunk.NewGRPCClientProxy(endpointName, makeDemoClient, shardMap, c)

	return &liffProxy{
		localLiff:   localLiff,
		shardFromID: shardFunction,
		clientProxy: proxy,
	}
}

func (lp *liffProxy) Liff(ctx context.Context, req *demo.LiffRequest) (*demo.LiffResponse, error) {
	// Get the shard from the identifier in the request. Each request must
	// contain the identifier you'll be sharding on. Typically this will be
	// entity keys, hashes or other identifiers that can be used to spread the
	// load across a set of nodes. Here it's a simple ID that the client sets
	// randomly.
	shard := lp.shardFromID(req.ID)

	// Retrieve the client matching the shard. If the shard is handled
	// locally both client and err is set to nil.
	client, err := lp.clientProxy.GetProxyClient(shard)
	if err != nil {
		// Something went wrong, either creating the client or connecting
		// to the node. Just return the error.
		return nil, err
	}

	if client != nil {
		// If the returned client is non-nil we are supposed to proxy the
		// request to one of the other nodes. It's as simple as this for gRPC.
		return client.(demo.DemoServiceClient).Liff(ctx, req)
	}

	// Both client and error is nil here so we are supposed to process this
	// request locally.
	return lp.localLiff.Liff(ctx, req)
}

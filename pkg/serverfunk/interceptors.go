package serverfunk

import (
	"context"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// LocalShard is a return value to the ShardConversionFunc if you want processing
// to be on the local node. With a round robin load balancer this will be evenly
// distributed on all nodes.
const LocalShard = -1

// ShardConversionFunc is the shard conversion function, ie return a shard based
// on the request parameter to the gRPC server methods. It will also return the
// expected response object for the request. If the shard is a negative value the
// local node is used.
type ShardConversionFunc func(request interface{}) (shard int, response interface{})

// WithClusterFunk returns server options for Clusterfunk gRPC servers. This
// will add a stream and unary interceptor to the server that will proxy the
// requests to the correct peer. The metrics string is the one used in the
// Parameters struct from the funk package.
// Streams are not proxied.
func WithClusterFunk(cluster funk.Cluster, shardFn ShardConversionFunc, clientProxy *ProxyConnections, metricsType string) []grpc.ServerOption {
	m := metrics.NewSinkFromString(metricsType, cluster)
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(createClusterfunkUnaryInterceptor(cluster.NodeID(), shardFn, clientProxy, m)),
	}
}

func createClusterfunkUnaryInterceptor(localID string, shardFn ShardConversionFunc, clientProxy *ProxyConnections, m metrics.Sink) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		shard, ret := shardFn(req)
		if shard >= 0 {
			client, nodeID, err := clientProxy.GetConnection(shard)
			if err != nil {
				return nil, err
			}
			if client != nil {
				// Change metadata to outgoing metadata since it won't be valid for the clients.
				md, ok := metadata.FromIncomingContext(ctx)
				outCtx := ctx
				if ok {
					outCtx = metadata.NewOutgoingContext(ctx, md)
				}
				err := client.Invoke(outCtx, info.FullMethod, req, ret)
				m.LogRequest(nodeID, info.FullMethod)
				return ret, err
			}
		}
		ret, err := handler(ctx, req)
		m.LogRequest(localID, info.FullMethod)
		return ret, err
	}
}

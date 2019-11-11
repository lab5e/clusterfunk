package serverfunk

import (
	"context"

	"google.golang.org/grpc"
)

// ShardConversionFunc is the shard conversion function, ie return a shard based
// on the request parameter to the gRPC server methods. It will also return the
// expected response object for the request. This *does* require a un
type ShardConversionFunc func(request interface{}) (shard int, response interface{})

// WithClusterFunk returns server options for Clusterfunk gRPC servers. This
// will add a stream and unary interceptor to the server that will proxy the
// requests to the correct peer.
// Streams are not proxied.
func WithClusterFunk(shardFn ShardConversionFunc, clientProxy *ProxyConnections) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StreamInterceptor(createClusterfunkStreamInterceptor(shardFn, clientProxy)),
		grpc.UnaryInterceptor(createClusterfunkUnaryInterceptor(shardFn, clientProxy)),
	}
}

func createClusterfunkUnaryInterceptor(shardFn ShardConversionFunc, clientProxy *ProxyConnections) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		shard, ret := shardFn(req)
		client, err := clientProxy.GetConnection(shard)
		if err != nil {
			return nil, err
		}
		if client != nil {
			err := client.Invoke(ctx, info.FullMethod, req, ret)
			return ret, err
		}

		return handler(ctx, req)
	}
}

func createClusterfunkStreamInterceptor(shardFn ShardConversionFunc, clientProxy *ProxyConnections) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// TODO: Handle later
		return handler(srv, ss)
	}
}

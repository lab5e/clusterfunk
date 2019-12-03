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
	"context"

	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/funk/metrics"

	"google.golang.org/grpc"
)

// ShardConversionFunc is the shard conversion function, ie return a shard based
// on the request parameter to the gRPC server methods. It will also return the
// expected response object for the request. This *does* require a un
type ShardConversionFunc func(request interface{}) (shard int, response interface{})

// WithClusterFunk returns server options for Clusterfunk gRPC servers. This
// will add a stream and unary interceptor to the server that will proxy the
// requests to the correct peer. The metrics string is the one used in the
// Parameters struct from the funk package.
// Streams are not proxied.
func WithClusterFunk(cluster funk.Cluster, shardFn ShardConversionFunc, clientProxy *ProxyConnections, metricsType string) []grpc.ServerOption {
	m := metrics.NewSinkFromString(metricsType)
	return []grpc.ServerOption{
		grpc.StreamInterceptor(createClusterfunkStreamInterceptor(cluster.NodeID(), shardFn, clientProxy, m)),
		grpc.UnaryInterceptor(createClusterfunkUnaryInterceptor(cluster.NodeID(), shardFn, clientProxy, m)),
	}
}

func createClusterfunkUnaryInterceptor(localID string, shardFn ShardConversionFunc, clientProxy *ProxyConnections, m metrics.Sink) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		shard, ret := shardFn(req)
		client, nodeID, err := clientProxy.GetConnection(shard)
		if err != nil {
			return nil, err
		}
		if client != nil {
			err := client.Invoke(ctx, info.FullMethod, req, ret)
			m.LogRequest(localID, nodeID, info.FullMethod)
			return ret, err
		}
		ret, err = handler(ctx, req)
		m.LogRequest(localID, localID, info.FullMethod)
		return ret, err
	}
}

func createClusterfunkStreamInterceptor(localID string, shardFn ShardConversionFunc, clientProxy *ProxyConnections, m metrics.Sink) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// TODO: Handle later
		return handler(srv, ss)
	}
}

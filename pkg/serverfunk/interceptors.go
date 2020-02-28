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

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/metrics"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
		logrus.
			WithField("method", info.FullMethod).
			WithField("clientStrem", info.IsClientStream).
			WithField("serverstream", info.IsServerStream).
			Info("Stream interceptor")

		// Use the server metadata to route according to sharding.
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			logrus.Error()
		}
		return handler(srv, ss)
	}
}

/*
type serverStreamWrapper struct {
	shardFn     ShardConversionFunc
	clientProxy *ProxyConnections
	original    grpc.ServerStream
	newstream   grpc.ClientStream
	streamDesc  *grpc.StreamDesc
	method      string
	opts        []grpc.CallOption
	local       bool
}

// SetHeader sets the header metadata. It may be called multiple times.
// When call multiple times, all the provided metadata will be merged.
// All the metadata will be sent out when one of the following happens:
//  - ServerStream.SendHeader() is called;
//  - The first response is sent out;
//  - An RPC status is sent out (error or success).
func (s *serverStreamWrapper) SetHeader(md metadata.MD) error {
	if s.local {
		return s.original.SetHeader(md)
	}
	logrus.Warning("Dropping SetHeader call")
	return nil
}

// SendHeader sends the header metadata.
// The provided md and headers set by SetHeader() will be sent.
// It fails if called multiple times.
func (s *serverStreamWrapper) SendHeader(md metadata.MD) error {
	if s.local {
		return s.original.SendHeader(md)
	}
	logrus.Warning("Dropping SendHeader call")
	return nil
}

// SetTrailer sets the trailer metadata which will be sent with the RPC status.
// When called more than once, all the provided metadata will be merged.
func (s *serverStreamWrapper) SetTrailer(md metadata.MD) {
	if s.local {
		s.original.SetTrailer(md)
		return
	}
	logrus.Warning("Dropping SetTrailer call")
}

// Context returns the context for this stream.
func (s *serverStreamWrapper) Context() context.Context {
	if s.newstream != nil {
		return s.newstream.Context()
	}
	return s.original.Context()
}

// SendMsg sends a message. On error, SendMsg aborts the stream and the
// error is returned directly.
//
// SendMsg blocks until:
//   - There is sufficient flow control to schedule m with the transport, or
//   - The stream is done, or
//   - The stream breaks.
//
// SendMsg does not wait until the message is received by the client. An
// untimely stream closure may result in lost messages.
//
func (s *serverStreamWrapper) SendMsg(m interface{}) error {
	if s.newstream != nil {
		logrus.Info("Forwarding send to client stream")
		return s.newstream.SendMsg(m)
	}
	if s.local {
		logrus.Info("Processing local")
		return s.original.SendMsg(m)
	}
	logrus.WithField("msg", m).Warning("Dropping message from server")
	return nil
}

// RecvMsg blocks until it receives a message into m or the stream is
// done. It returns io.EOF when the client has performed a CloseSend. On
// any non-EOF error, the stream is aborted and the error contains the
// RPC status.
func (s *serverStreamWrapper) RecvMsg(m interface{}) error {
	if s.newstream != nil {
		// Forward to client
		logrus.WithField("msg", m).Info("Forwarding recv to client stream")
		return s.newstream.RecvMsg(m)
	}
	if s.local {
		logrus.Info("Processing local recvmsg")
		return s.original.RecvMsg(m)
	}
	err := s.original.RecvMsg(m)
	if err != nil {
		return err
	}
	// We now have a message from the client and we can use the shard function
	// to find the correct server we should stream from. The return struct is
	// ignored since this is an unary call
	shard, _ := s.shardFn(m)
	client, nodeID, err := s.clientProxy.GetConnection(shard)
	logrus.WithField("nodeID", nodeID).Info("Proxy stream to node")
	if err != nil {
		return err
	}
	if client != nil {
		// Create a new stream and replay the operations on the stream
		var err error
		s.newstream, err = client.NewStream(s.original.Context(), s.streamDesc, s.method, s.opts...)
		if err != nil {
			return err
		}
		// Forward the message to the client
		return s.newstream.RecvMsg(m)
	}
	// Local stream
	logrus.Info("Using local stream")
	s.local = true
	return nil
}
*/

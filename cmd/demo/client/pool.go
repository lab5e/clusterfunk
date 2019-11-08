package main

import (
	"context"

	"google.golang.org/grpc"
)

type ClientfunkConnectionPool interface {
	// Connection returns a dummy collection you should use to create the
	// gRPC client. This connection might not work with gRPC connections but
	// will be replaced with another connection in the interceptor.
	Connection() *grpc.ClientConn

	// Interceptors returns stream and unary interceptors to one of the
	// cluster nodes based on the request.
	ClusterDialOptions() []grpc.DialOption
}

func NewClientfunkConnectionPool() ClientfunkConnectionPool {
	return &funkClientPool{}
}

type funkClientPool struct {
}

func (f *funkClientPool) Connection() *grpc.ClientConn {
	return nil
}

func (f *funkClientPool) ClusterDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(f.unaryInterceptor),
		grpc.WithStreamInterceptor(f.streamInterceptor),
	}
}

func (f *funkClientPool) unaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return invoker(ctx, method, req, reply, cc, opts...)
}

func (f *funkClientPool) streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return streamer(ctx, desc, cc, method, opts...)
}

package main

/*
// TODO: docs, name

// ClientfunkConnectionPool is a client pool for gRPC connections to the cluster.
// The connections are replaced in interceptors on the client connection.
type ClientfunkConnectionPool interface {
	// Connection returns a dummy collection you should use to create the
	// gRPC client. This connection might not work with gRPC connections but
	// will be replaced with another connection in the interceptor.
	Connection() *grpc.ClientConn

	// Interceptors returns stream and unary interceptors to one of the
	// cluster nodes based on the request.
	ClusterDialOptions() []grpc.DialOption
}

// NewClientfunkConnectionPool creates a new ClientFunkConnectionPool instance.
func NewClientfunkConnectionPool(pool clientfunk.Pool) ClientfunkConnectionPool {
	return &funkClientPool{Pool: pool}

}

type funkClientPool struct {
	Pool            clientfunk.Pool
	DummyConnection *grpc.ClientConn
}

func (f *funkClientPool) Connection() *grpc.ClientConn {
	return f.DummyConnection
}

func (f *funkClientPool) ClusterDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(f.unaryInterceptor),
		grpc.WithStreamInterceptor(f.streamInterceptor),
	}
}

func (f *funkClientPool) unaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if cc != f.DummyConnection {
		panic("This is not the connection I expected")
	}
	conn, err := f.Pool.Take(ctx)
	if err != nil {
		return err
	}
	defer f.Pool.Release(conn)
	return invoker(ctx, method, req, reply, conn, opts...)
}

func (f *funkClientPool) streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if cc != f.DummyConnection {
		panic("This is not the connection I expected")
	}
	conn, err := f.Pool.Take(ctx)
	if err != nil {
		return nil, err
	}
	defer f.Pool.Release(conn)
	return streamer(ctx, desc, conn, method, opts...)
}
*/

package clientfunk

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc"
)

const maxPoolSize = 100

// ClientPool is a simple pool for gRPC client connections. The pool is set
// initially with a set of endpoints and they are handed out round robin style.
// Client connections can be handed back or marked unhealthy. When a connection
// is marked as unhealthy it is closed and dropped and never handed out
// again. The pool can be refreshed with new connections regularly.
type ClientPool struct {
	connections chan *grpc.ClientConn
	DialOptions []grpc.DialOption
	taken       *int32
}

// NewClientPool creates a new EndpointPool instance
func NewClientPool(options []grpc.DialOption) *ClientPool {
	return &ClientPool{
		connections: make(chan *grpc.ClientConn, maxPoolSize),
		DialOptions: options[:],
		taken:       new(int32),
	}
}

// Take returns a client connection from the pool.
func (c *ClientPool) Take(ctx context.Context) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ret := <-c.connections:
		atomic.AddInt32(c.taken, 1)
		return ret, nil
	}
}

// Release releases the endpoint back into the pool. There's no check for
// new connections in the pool so
func (c *ClientPool) Release(conn *grpc.ClientConn) {
	c.connections <- conn
	atomic.AddInt32(c.taken, -1)
}

// MarkUnhealthy removes the connection from the pool
func (c *ClientPool) MarkUnhealthy(conn *grpc.ClientConn) {
	conn.Close()
	atomic.AddInt32(c.taken, -1)
}

// Sync refreshes the connections in the pool. Note that existing healthy connections
// will still be served until they've been consumed at this point.  This method might block if
// more than
func (c *ClientPool) Sync(endpoints []string) {
	if len(endpoints) > maxPoolSize {
		panic("max client pool size will be exceeded")
	}
	for _, v := range endpoints {
		conn, err := grpc.Dial(v, c.DialOptions...)
		if err != nil {
			continue
		}
		c.connections <- conn
	}
}

// Available returns the number of available connections. Mostly for diagnostics.
func (c *ClientPool) Available() int {
	return len(c.connections)
}

// Size returns the number of connections (theoretically) available, ie the
// number of available connections plus the number of connections taken.
func (c *ClientPool) Size() int {
	return len(c.connections) + int(atomic.LoadInt32(c.taken))
}

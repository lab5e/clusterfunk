package clientfunk

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc"
)

const maxPoolSize = 100

// Pool is a simple pool for gRPC client connections. The pool is set
// initially with a set of endpoints and they are handed out round robin style.
// Client connections can be handed back or marked unhealthy. When a connection
// is marked as unhealthy it is closed and dropped and never handed out
// again. The pool can be refreshed with new connections regularly.
type Pool struct {
	connections chan *grpc.ClientConn
	DialOptions []grpc.DialOption
	taken       *int32
}

// NewPool creates a new EndpointPool instance
func NewPool(options []grpc.DialOption) *Pool {
	return &Pool{
		connections: make(chan *grpc.ClientConn, maxPoolSize),
		DialOptions: options[:],
		taken:       new(int32),
	}
}

// Take returns a client connection from the pool.
func (c *Pool) Take(ctx context.Context) (*grpc.ClientConn, error) {
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
func (c *Pool) Release(conn *grpc.ClientConn) {
	c.connections <- conn
	atomic.AddInt32(c.taken, -1)
}

// MarkUnhealthy removes the connection from the pool
func (c *Pool) MarkUnhealthy(conn *grpc.ClientConn) {
	conn.Close()
	atomic.AddInt32(c.taken, -1)
}

// Sync refreshes the connections in the pool. Note that existing healthy connections
// will still be served until they've been consumed at this point.  This method might block if
// more than
func (c *Pool) Sync(endpoints []string) {
	if len(endpoints) > maxPoolSize {
		panic("max client pool size will be exceeded")
	}
	// Drain the pool
	for len(c.connections) > 0 {
		select {
		case conn := <-c.connections:
			conn.Close()
		default:
			// skip
		}
	}
	// ..and reestablish the new connections
	for _, v := range endpoints {
		conn, err := grpc.Dial(v, c.DialOptions...)
		if err != nil {
			continue
		}
		c.connections <- conn
	}
}

// Available returns the number of available connections. Mostly for diagnostics.
func (c *Pool) Available() int {
	return len(c.connections)
}

// Size returns the number of connections (theoretically) available, ie the
// number of available connections plus the number of connections taken.
func (c *Pool) Size() int {
	return len(c.connections) + int(atomic.LoadInt32(c.taken))
}

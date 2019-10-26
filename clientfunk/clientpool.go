package clientfunk

import (
	"context"
	"sync/atomic"

	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

const maxPoolSize = 100

// ClientPool is a simple pool for gRPC client connections. The pool is set
// initially with a set of endpoints and they are handed out round robin style.
// Client connections can be handed back or marked unhealthy. When a connection
// is marked as unhealthy it is closed and dropped and never handed out
// again. The pool can be refreshed with new connections regularly.
type ClientPool struct {
	currentEndpoints toolbox.StringSet
	connections      chan *grpc.ClientConn
	poolSize         *int32
	DialOptions      []grpc.DialOption
}

// NewClientPool creates a new EndpointPool instance
func NewClientPool(options []grpc.DialOption) *ClientPool {
	return &ClientPool{
		connections:      make(chan *grpc.ClientConn, maxPoolSize),
		DialOptions:      options[:],
		currentEndpoints: toolbox.NewStringSet(),
		poolSize:         new(int32),
	}
}

// Take returns a client connection from the pool.
func (c *ClientPool) Take(ctx context.Context) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ret := <-c.connections:
		return ret, nil
	}
}

// Release releases the endpoint back into the pool. There's no check for
// new connections in the pool so
func (c *ClientPool) Release(conn *grpc.ClientConn) {
	c.connections <- conn
}

// MarkUnhealthy removes the connection from the pool
func (c *ClientPool) MarkUnhealthy(conn *grpc.ClientConn) {
	c.currentEndpoints.Remove(conn.Target())
	conn.Close()
}

// Sync refreshes the connections in the pool. Note that existing healthy connections
// will still be served until they've been consumed at this point.  This method might block if
// more than
func (c *ClientPool) Sync(endpoints []string) {
	if len(endpoints) > maxPoolSize {
		panic("max client pool size will be exceeded")
	}
	for _, v := range endpoints {
		if c.currentEndpoints.Add(v) {
			conn, err := grpc.Dial(v, c.DialOptions...)
			if err != nil {
				c.currentEndpoints.Remove(v)
				continue
			}
			c.connections <- conn
		}
	}
	atomic.StoreInt32(c.poolSize, int32(c.currentEndpoints.Size()))
}

// LowWaterMark returns true if less than 1/3rd of the connections in the pool
// is unhealthy.
func (c *ClientPool) LowWaterMark() bool {
	size := atomic.LoadInt32(c.poolSize)
	if size == 0 || (float32(c.currentEndpoints.Size())/float32(size) < 0.333) {
		return true
	}
	return false
}

// Available returns the number of available connections. Mostly for diagnostics.
func (c *ClientPool) Available() int {
	return len(c.connections)
}

// Size returns the number of healthy connections in the pool.
func (c *ClientPool) Size() int {
	return c.currentEndpoints.Size()
}

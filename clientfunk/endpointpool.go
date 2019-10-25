package clientfunk

import (
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

type connection struct {
	Connection *grpc.ClientConn
	Endpoint   string
	Healthy    bool
	Taken      bool
}

func (c *connection) SetUnhealthy() {
	c.Healthy = false
	if c.Connection != nil {
		c.Connection.Close()
		c.Connection = nil
	}
}

// EndpointPool is a helper for gRPC clients. It keeps a pool of connections.
// Connections can be flagged as unhealthy. Connection management is done
// by the gRPC connections and if it is reported as unhealthy it will be closed.
type EndpointPool struct {
	Mutex       *sync.Mutex
	Connections []connection
	DialOptions []grpc.DialOption
}

// NewEndpointPool creates a new EndpointPool instance
func NewEndpointPool(options []grpc.DialOption) *EndpointPool {
	return &EndpointPool{
		Mutex:       &sync.Mutex{},
		Connections: make([]connection, 0),
		DialOptions: options[:],
	}
}

// GetEndpoint returns a random assumed healthy endpoint from the pool. An error
// is returned if the pool is exhausted.
func (e *EndpointPool) GetEndpoint() (*grpc.ClientConn, error) {

	for i, conn := range e.Connections {
		if !conn.Taken && conn.Healthy {
			if conn.Connection == nil {
				var err error
				e.Connections[i].Connection, err = grpc.Dial(conn.Endpoint, e.DialOptions...)
				if err != nil {
					log.WithError(err).Warning("Unable to dial existing connection")
					e.Connections[i].SetUnhealthy()
					continue
				}
			}
			e.Connections[i].Taken = true
			return e.Connections[i].Connection, nil
		}
	}
	return nil, errors.New("no connections available")
}

// ReleaseEndpoint releases the endpoint back into the pool. If the pool doesn't
// contain the endpoint it will be closed and discarded.
func (e *EndpointPool) ReleaseEndpoint(conn *grpc.ClientConn) bool {
	for i, conn := range e.Connections {
		if conn.Endpoint == conn.Connection.Target() {
			e.Connections[i].Taken = false
			return true
		}
	}
	return false
}

// MarkUnhealthy releases the connection back into the pool and marks it
// unhealthy. The connection will be closed. Connections that does not exist
// in the pool will also be closed.
func (e *EndpointPool) MarkUnhealthy(conn *grpc.ClientConn) bool {
	conn.Close()
	for i, c := range e.Connections {
		if c.Endpoint == conn.Target() {
			e.Connections[i].SetUnhealthy()
			return true
		}
	}
	return false
}

// clearUnhealthy removes all unhealthy connections from the pool
func (e *EndpointPool) clearUnhealthy() {
	for i, v := range e.Connections {
		if !v.Healthy {
			e.Connections = append(e.Connections[:i], e.Connections[i+1:]...)
		}
	}
}

// Sync refreshes the endpoints in the pool. All endpoints in the input is
// assumed to be healthy. Existing connections are kept, connections that no
// longer exists in the set of endpoints are removed and any new connections are
// added to the pool.
func (e *EndpointPool) Sync(endpoints []string) {
	ss := toolbox.NewStringSet()
	ss.Sync(endpoints...)

	for i, v := range e.Connections {
		if ss.Contains(v.Endpoint) {
			ss.Remove(v.Endpoint)
			e.Connections[i].Healthy = true
			continue
		}
		// This does not exist in the new. Mark as unhealthy.
		e.Connections[i].SetUnhealthy()
	}
	// ss now contains all new endpoints, all unhealthy endpoints should be
	// removed from this pool
	e.clearUnhealthy()
	for _, ep := range ss.Strings {
		e.Connections = append(e.Connections, connection{
			Healthy:    true,
			Connection: nil,
			Endpoint:   ep,
			Taken:      false,
		})
	}
}

// LowWaterMark returns true if less than 1/3rd of the connections in the pool
// is unhealthy.
func (e *EndpointPool) LowWaterMark() bool {
	healthy := 0.0
	if len(e.Connections) == 0 {
		return true
	}
	for _, v := range e.Connections {
		if v.Healthy {
			healthy++
		}
	}
	if healthy/float64(len(e.Connections)) < 0.333 {
		return true
	}
	return false
}

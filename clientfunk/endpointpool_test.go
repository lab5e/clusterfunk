package clientfunk

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stalehd/clusterfunk/funk/clusterproto"
	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

// This is a dummy server. It's the simplest server in the library so we'll use that.
type dummyserver struct {
}

func (d *dummyserver) ConfirmShardMap(context.Context, *clusterproto.ConfirmShardMapRequest) (*clusterproto.ConfirmShardMapResponse, error) {
	return nil, errors.New("not implemented")
}

const poolElements = 10

func TestEndpointPool(t *testing.T) {
	assert := require.New(t)

	pool := NewEndpointPool([]grpc.DialOption{grpc.WithInsecure()})
	// Make a pool of 10 servers. These are just to avoid errors
	var endpoints []string

	for i := 0; i < poolElements; i++ {
		ep := toolbox.RandomLocalEndpoint()
		endpoints = append(endpoints, ep)
		go func() {
			srv := grpc.NewServer()
			clusterproto.RegisterClusterLeaderServiceServer(srv, &dummyserver{})
			listener, err := net.Listen("tcp", ep)
			assert.NoError(err)
			assert.NoError(srv.Serve(listener))
		}()
	}
	assert.True(pool.LowWaterMark(), "Empty pool should have low watermark")
	pool.Sync(endpoints)
	assert.Falsef(pool.LowWaterMark(), "Expected no low watermark")
	var taken []*grpc.ClientConn

	for i := 0; i < poolElements; i++ {
		c, err := pool.GetEndpoint()
		assert.NoError(err)
		assert.NotNil(c)
		taken = append(taken, c)
	}

	_, err := pool.GetEndpoint()
	assert.Error(err, "Expected error when using exhausted pool")

	// Release all
	for _, v := range taken {
		assert.True(pool.ReleaseEndpoint(v))
	}
	taken = []*grpc.ClientConn{}
	c, err := pool.GetEndpoint()
	assert.NoError(err, "Expected no error when pool is full again")
	pool.MarkUnhealthy(c)

	for i := 0; i < poolElements/2; i++ {
		c, err := pool.GetEndpoint()
		assert.NoErrorf(err, "Did not expect error when retrieving connection: %+v", pool.Connections)
		pool.MarkUnhealthy(c)
	}

	assert.False(pool.LowWaterMark(), "Expecting no low watermark when 50% is unhealthy")
}

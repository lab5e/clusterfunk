package clientfunk

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stalehd/clusterfunk/toolbox"
	"google.golang.org/grpc"
)

const poolElements = 20

func TestEndpointPool(t *testing.T) {
	assert := require.New(t)

	pool := NewEndpointPool([]grpc.DialOption{grpc.WithInsecure()})
	// Make a pool of 10 servers. These are just to avoid errors
	var endpoints []string

	// Note that the gRPC connections will not connect to a server
	// when they are created; ie no need for a server runnin gin the other end.
	for i := 0; i < poolElements; i++ {
		ep := toolbox.RandomLocalEndpoint()
		endpoints = append(endpoints, ep)
	}
	assert.Equal(0, pool.Available())
	assert.True(pool.LowWaterMark(), "Empty pool should have low watermark")
	pool.Sync(endpoints)
	assert.Equal(poolElements, pool.Available())
	assert.Falsef(pool.LowWaterMark(), "Expected no low watermark")
	var taken []*grpc.ClientConn

	for i := 0; i < poolElements; i++ {
		c, err := pool.GetEndpoint()
		assert.NoError(err)
		assert.NotNil(c)
		taken = append(taken, c)
	}
	assert.Equal(0, pool.Available())

	_, err := pool.GetEndpoint()
	assert.Error(err, "Expected error when using exhausted pool")

	// Release all
	for i, v := range taken {
		t.Logf("Releasing %s", v.Target())
		assert.Equal(i, pool.Available())
		assert.True(pool.ReleaseEndpoint(v))
	}
	assert.Equal(poolElements, pool.Available())

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

	// Re-sync with half the number of connections
	endpoints = endpoints[:poolElements/2]
	pool.Sync(endpoints)
	assert.Equal(len(endpoints), pool.Available())

	for range endpoints {
		c, _ := pool.GetEndpoint()
		pool.MarkUnhealthy(c)
	}
	assert.True(pool.LowWaterMark())
}

func BenchmarkPoolGetRelease(b *testing.B) {
	pool := NewEndpointPool([]grpc.DialOption{grpc.WithInsecure()})
	// Make a pool of 10 servers. These are just to avoid errors
	var endpoints []string

	// Note that the gRPC connections will not connect to a server
	// when they are created; ie no need for a server runnin gin the other end.
	for i := 0; i < poolElements; i++ {
		ep := toolbox.RandomLocalEndpoint()
		endpoints = append(endpoints, ep)
	}
	pool.Sync(endpoints)

	// Make it slightly realistic -- mark 1/3 as unhealthy
	for range endpoints {
		if rand.Int()%3 == 0 {
			c, _ := pool.GetEndpoint()
			pool.MarkUnhealthy(c)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := pool.GetEndpoint()
		pool.ReleaseEndpoint(c)
	}
}

func BenchmarkPoolSync(b *testing.B) {
	pool := NewEndpointPool([]grpc.DialOption{grpc.WithInsecure()})
	// Make a pool of 10 servers. These are just to avoid errors
	var endpoints []string

	// Note that the gRPC connections will not connect to a server
	// when they are created; ie no need for a server runnin gin the other end.
	for i := 0; i < poolElements; i++ {
		ep := toolbox.RandomLocalEndpoint()
		endpoints = append(endpoints, ep)
	}

	pool.Sync(endpoints)
	b.ResetTimer()

	// This might take a loooooong time to run since we're stopping
	// the timer and doing slow work before restarting.
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		endpoints = endpoints[:poolElements/3]
		for i := 0; i < poolElements/3; i++ {
			endpoints = append(endpoints, toolbox.RandomLocalEndpoint())
		}
		b.StartTimer()
		pool.Sync(endpoints)
	}
}

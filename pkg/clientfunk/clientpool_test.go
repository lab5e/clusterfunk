package clientfunk

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stalehd/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

const poolElements = 20

func TestEndpointPool(t *testing.T) {
	assert := require.New(t)

	pool := NewPool([]grpc.DialOption{grpc.WithInsecure()})
	// Make a pool of 10 servers. These are just to avoid errors
	var endpoints []string

	// Note that the gRPC connections will not connect to a server
	// when they are created; ie no need for a server runnin gin the other end.
	for i := 0; i < poolElements; i++ {
		ep := toolbox.RandomLocalEndpoint()
		endpoints = append(endpoints, ep)
	}
	assert.Equal(0, pool.Available())
	pool.Sync(endpoints)
	assert.Equal(poolElements, pool.Available())

	var taken []*grpc.ClientConn

	for i := 0; i < poolElements; i++ {
		ctx, done := context.WithTimeout(context.Background(), 10*time.Millisecond)
		c, err := pool.Take(ctx)
		assert.NoError(err)
		assert.NotNil(c)
		taken = append(taken, c)
		done()
	}
	assert.Equal(0, pool.Available())

	ctx, done := context.WithTimeout(context.Background(), 10*time.Millisecond)
	_, err := pool.Take(ctx)
	assert.Error(err, "Expected error when using exhausted pool")
	done()

	// Release all
	for i, v := range taken {
		t.Logf("Releasing %s", v.Target())
		assert.Equal(i, pool.Available())
		pool.Release(v)
	}
	assert.Equal(poolElements, pool.Available())

	ctx, done = context.WithTimeout(context.Background(), 10*time.Millisecond)
	c, err := pool.Take(ctx)
	assert.NoError(err, "Expected no error when pool is full again")
	pool.MarkUnhealthy(c)
	done()
	for i := 0; i < poolElements-1; i++ {
		ctx, done := context.WithTimeout(context.Background(), 10*time.Millisecond)
		c, err := pool.Take(ctx)
		assert.NoError(err, "Did not expect error when retrieving connection")
		pool.MarkUnhealthy(c)
		done()
	}
	assert.Equal(0, pool.Available())

	// Re-sync with half the number of connections. All are marked unhealthy so
	// the only endpoints available will be the new ones
	endpoints = endpoints[:poolElements/2]
	pool.Sync(endpoints)
	assert.Equal(len(endpoints), pool.Available())

	for range endpoints {
		ctx, done := context.WithTimeout(context.Background(), 10*time.Millisecond)
		c, _ := pool.Take(ctx)
		pool.MarkUnhealthy(c)
		done()
	}
	assert.Equal(0, pool.Available())
}

func BenchmarkPoolGetRelease(b *testing.B) {
	pool := NewPool([]grpc.DialOption{grpc.WithInsecure()})
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
			ctx := context.Background()
			c, _ := pool.Take(ctx)
			pool.MarkUnhealthy(c)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		c, _ := pool.Take(ctx)
		pool.Release(c)
	}
}

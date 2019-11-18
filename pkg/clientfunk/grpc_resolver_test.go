package clientfunk

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

func TestAddRemoveEndpoints(t *testing.T) {
	assert := require.New(t)

	assert.Len(cfEndpoints["ep.test"], 0)
	AddClusterEndpoint("ep.test", "example.com:1234")
	assert.Contains(cfEndpoints["ep.test"], resolver.Address{Addr: "example.com:1234"})
	AddClusterEndpoint("ep.test", "example.com:4321")
	assert.Len(cfEndpoints["ep.test"], 2)

	RemoveClusterEndpoint("ep.none", "example.com:1234")
	RemoveClusterEndpoint("ep.none", "example.com:1234")

	RemoveClusterEndpoint("ep.test", "example.com:1234")
	assert.Len(cfEndpoints["ep.test"], 1)
	RemoveClusterEndpoint("ep.test", "example.com:4321")
	assert.Len(cfEndpoints["ep.test"], 0)

	UpdateClusterEndpoints("ep.test", []string{"example.com:1", "example.com:2", "example.com:3"})
	assert.Len(cfEndpoints["ep.test"], 3)
	UpdateClusterEndpoints("ep.test", []string{})
	assert.Len(cfEndpoints["ep.test"], 0)
}

func TestClusterResolverBuilder(t *testing.T) {
	assert := require.New(t)

	delete(cfEndpoints, "ep.test")
	AddClusterEndpoint("ep.test", "example.com:1234")

	conn, err := grpc.Dial("cluster:///ep.test", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(GRPCServiceConfig))
	assert.NoError(err)
	assert.NotNil(conn)
	defer conn.Close()

	_, err = grpc.Dial("cluster:///xep.test", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(GRPCServiceConfig))
	assert.Error(err)
}

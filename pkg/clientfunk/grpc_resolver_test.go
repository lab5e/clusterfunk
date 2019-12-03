package clientfunk
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
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

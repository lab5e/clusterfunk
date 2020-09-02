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
	"context"
	"errors"
	"net"
	"testing"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/grpcutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestEndpointLookup(t *testing.T) {
	assert := require.New(t)
	server := grpc.NewServer()

	managepb.RegisterClusterManagementServer(server, &dummyManagement{})
	ep := toolbox.RandomLocalEndpoint()

	go func() {
		listener, err := net.Listen("tcp", ep)
		assert.NoError(err)
		assert.NoError(server.Serve(listener))
	}()

	eps, err := GetEndpoints("ep.test", grpcutil.GRPCClientParam{ServerEndpoint: ep})
	assert.NoError(err)
	assert.Contains(eps, "127.1.2.3:1234")
	assert.Contains(eps, "127.4.3.2:4321")
	assert.Len(eps, 2)

	_, err = GetEndpoints("ep.err", grpcutil.GRPCClientParam{ServerEndpoint: ep})
	assert.Error(err)

	_, err = GetEndpoints("ep.test", grpcutil.GRPCClientParam{ServerEndpoint: toolbox.RandomLocalEndpoint()})
	assert.Error(err)

}

// Dummy gRPC test server
type dummyManagement struct {
}

func (d *dummyManagement) GetStatus(context.Context, *managepb.GetStatusRequest) (*managepb.GetStatusResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) ListNodes(context.Context, *managepb.ListNodesRequest) (*managepb.ListNodesResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) FindEndpoint(ctx context.Context, req *managepb.EndpointRequest) (*managepb.EndpointResponse, error) {
	if req.EndpointName == "ep.err" {
		return &managepb.EndpointResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_GENERIC,
				Message:   "Something went wrong",
			},
		}, nil
	}
	return &managepb.EndpointResponse{
		Endpoints: []*managepb.EndpointInfo{
			&managepb.EndpointInfo{Name: "ep.test", HostPort: "127.1.2.3:1234"},
			&managepb.EndpointInfo{Name: "ep.test", HostPort: "127.4.3.2:4321"},
		},
	}, nil
}

func (d *dummyManagement) ListEndpoints(ctx context.Context, req *managepb.ListEndpointRequest) (*managepb.ListEndpointResponse, error) {
	return &managepb.ListEndpointResponse{
		Endpoints: []*managepb.EndpointInfo{
			&managepb.EndpointInfo{Name: "ep.a", HostPort: "127.1.2.3:1234"},
			&managepb.EndpointInfo{Name: "ep.b", HostPort: "127.4.3.2:4321"},
		},
	}, nil
}

func (d *dummyManagement) AddNode(context.Context, *managepb.AddNodeRequest) (*managepb.AddNodeResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) RemoveNode(context.Context, *managepb.RemoveNodeRequest) (*managepb.RemoveNodeResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) StepDown(context.Context, *managepb.StepDownRequest) (*managepb.StepDownResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) ListShards(context.Context, *managepb.ListShardsRequest) (*managepb.ListShardsResponse, error) {
	return nil, errors.New("not implemented")
}

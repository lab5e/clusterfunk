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

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/clustermgmt"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestEndpointLookup(t *testing.T) {
	assert := require.New(t)
	server := grpc.NewServer()

	clustermgmt.RegisterClusterManagementServer(server, &dummyManagement{})
	ep := toolbox.RandomLocalEndpoint()

	go func() {
		listener, err := net.Listen("tcp", ep)
		assert.NoError(err)
		assert.NoError(server.Serve(listener))
	}()

	eps, err := GetEndpoints("ep.test", toolbox.GRPCClientParam{ServerEndpoint: ep})
	assert.NoError(err)
	assert.Contains(eps, "127.1.2.3:1234")
	assert.Contains(eps, "127.4.3.2:4321")
	assert.Len(eps, 2)

	_, err = GetEndpoints("ep.err", toolbox.GRPCClientParam{ServerEndpoint: ep})
	assert.Error(err)

	_, err = GetEndpoints("ep.test", toolbox.GRPCClientParam{ServerEndpoint: toolbox.RandomLocalEndpoint()})
	assert.Error(err)

}

// Dummy gRPC test server
type dummyManagement struct {
}

func (d *dummyManagement) GetStatus(context.Context, *clustermgmt.GetStatusRequest) (*clustermgmt.GetStatusResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) ListNodes(context.Context, *clustermgmt.ListNodesRequest) (*clustermgmt.ListNodesResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) FindEndpoint(ctx context.Context, req *clustermgmt.EndpointRequest) (*clustermgmt.EndpointResponse, error) {
	if req.EndpointName == "ep.err" {
		return &clustermgmt.EndpointResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_GENERIC,
				Message:   "Something went wrong",
			},
		}, nil
	}
	return &clustermgmt.EndpointResponse{
		Endpoints: []*clustermgmt.EndpointInfo{
			&clustermgmt.EndpointInfo{Name: "ep.test", HostPort: "127.1.2.3:1234"},
			&clustermgmt.EndpointInfo{Name: "ep.test", HostPort: "127.4.3.2:4321"},
		},
	}, nil
}

func (d *dummyManagement) ListEndpoints(ctx context.Context, req *clustermgmt.ListEndpointRequest) (*clustermgmt.ListEndpointResponse, error) {
	return &clustermgmt.ListEndpointResponse{
		Endpoints: []*clustermgmt.EndpointInfo{
			&clustermgmt.EndpointInfo{Name: "ep.a", HostPort: "127.1.2.3:1234"},
			&clustermgmt.EndpointInfo{Name: "ep.b", HostPort: "127.4.3.2:4321"},
		},
	}, nil
}

func (d *dummyManagement) AddNode(context.Context, *clustermgmt.AddNodeRequest) (*clustermgmt.AddNodeResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) RemoveNode(context.Context, *clustermgmt.RemoveNodeRequest) (*clustermgmt.RemoveNodeResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) StepDown(context.Context, *clustermgmt.StepDownRequest) (*clustermgmt.StepDownResponse, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyManagement) ListShards(context.Context, *clustermgmt.ListShardsRequest) (*clustermgmt.ListShardsResponse, error) {
	return nil, errors.New("not implemented")
}

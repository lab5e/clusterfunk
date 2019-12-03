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
	"time"

	"github.com/stalehd/clusterfunk/pkg/funk/clustermgmt"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

// GetEndpoints returns the endpoints in the cluster. This method uses the
// gRPC management endpoint to query for endpoints.
func GetEndpoints(name string, managementClientParam toolbox.GRPCClientParam) ([]string, error) {
	opts, err := toolbox.GetGRPCDialOpts(managementClientParam)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(managementClientParam.ServerEndpoint, opts...)
	if err != nil {
		return nil, err
	}

	client := clustermgmt.NewClusterManagementClient(conn)

	// TODO: use parameter for timeout. Maybe include in grpc client params. Defaults 1s
	ctx, done := context.WithTimeout(context.Background(), 1*time.Second)
	defer done()
	res, err := client.FindEndpoint(ctx, &clustermgmt.EndpointRequest{
		EndpointName: name,
	})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, errors.New(res.Error.Message)
	}

	var ret []string
	for _, v := range res.Endpoints {
		ret = append(ret, v.HostPort)
	}
	return ret, nil
}

package clientfunk

import (
	"context"
	"errors"
	"time"

	"github.com/stalehd/clusterfunk/pkg/funk/clustermgmt"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

// GetAllEndpoints returns all endpoints registered in the cluster
func GetAllEndpoints(managementClientParam toolbox.GRPCClientParam) ([]string, error) {
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
	res, err := client.ListEndpoints(ctx, &clustermgmt.ListEndpointRequest{})
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

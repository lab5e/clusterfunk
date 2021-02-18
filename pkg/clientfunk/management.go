package clientfunk

import (
	"errors"
	"fmt"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/gotoolbox/grpcutil"
)

// ConnectToManagement connects to the management endpoint by looking up a
// management endpoint via the client. This is a convenience method
func ConnectToManagement(client Client) (managepb.ClusterManagementClient, error) {
	fmt.Println("Waiting for endpoints...")
	client.WaitForEndpoint(funk.ManagementEndpoint)
	for _, ep := range client.Endpoints() {
		if ep.Name == funk.ManagementEndpoint {
			grpcParams := grpcutil.GRPCClientParam{
				ServerEndpoint:     ep.ListenAddress,
				TLS:                false,
				CAFile:             "",
				ServerHostOverride: "",
			}
			conn, err := grpcutil.NewGRPCClientConnection(grpcParams)
			if err != nil {
				return nil, fmt.Errorf("could not dial management endpoint for cluster: %v", err)

			}
			return managepb.NewClusterManagementClient(conn), nil

		}
	}
	fmt.Println("Could not find any endpoints")
	return nil, errors.New("no management endpoints found")

}

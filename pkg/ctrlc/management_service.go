package ctrlc

import (
	"fmt"
	"os"
	"time"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/clientfunk"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/managepb"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

const gRPCTimeout = 10 * time.Second

func connectToManagement(params ManagementServerParameters) managepb.ClusterManagementClient {
	if params.Endpoint == "" && params.Zeroconf {
		if params.ClusterName == "" {
			fmt.Fprintf(os.Stderr, "Needs a cluster name if zeroconf is to be used for discovery")
			return nil
		}
		ep, err := clientfunk.ZeroconfManagementLookup(params.ClusterName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to do zeroconf lookup: %v\n", err)
			return nil
		}
		params.Endpoint = ep
	}

	if params.Endpoint == "" {
		fmt.Fprintf(os.Stderr, "Need an endpoint for one of the cluster nodes")
		return nil
	}

	grpcParams := toolbox.GRPCClientParam{
		ServerEndpoint:     params.Endpoint,
		TLS:                params.TLS,
		CAFile:             params.CertFile,
		ServerHostOverride: params.HostnameOverride,
	}
	opts, err := toolbox.GetGRPCDialOpts(grpcParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create GRPC dial options: %v\n", err)
		return nil
	}
	conn, err := grpc.Dial(grpcParams.ServerEndpoint, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not dial management endpoint: %v\n", err)
		return nil
	}
	return managepb.NewClusterManagementClient(conn)

}

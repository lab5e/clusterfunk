package ctrlc

import (
	"fmt"
	"os"
	"time"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/gotoolbox/grpcutil"
	"google.golang.org/grpc"
)

const gRPCTimeout = 10 * time.Second

func connectToManagement(params ManagementServerParameters) managepb.ClusterManagementClient {
	if params.Endpoint == "" && params.Zeroconf {
		if params.Name == "" {
			fmt.Fprintf(os.Stderr, "Needs a cluster name if zeroconf is to be used for discovery")
			return nil
		}
		ep, err := clientfunk.ZeroconfManagementLookup(params.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Zeroconf lookup error when searching for cluster %s: %v\n", params.Name, err)
			return nil
		}
		params.Endpoint = ep
	}

	if params.Endpoint == "" {
		fmt.Fprintf(os.Stderr, "Need an endpoint for one of the cluster nodes")
		return nil
	}

	grpcParams := grpcutil.GRPCClientParam{
		ServerEndpoint:     params.Endpoint,
		TLS:                params.TLS,
		CAFile:             params.CertFile,
		ServerHostOverride: params.HostnameOverride,
	}
	opts, err := grpcutil.GetDialOpts(grpcParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create GRPC dial options: %v\n", err)
		return nil
	}
	conn, err := grpc.Dial(grpcParams.ServerEndpoint, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not dial management endpoint for cluster %s. Is is it available? : %v\n", params.Name, err)
		return nil
	}
	return managepb.NewClusterManagementClient(conn)

}

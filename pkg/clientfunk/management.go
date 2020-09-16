package clientfunk

import (
	"errors"
	"fmt"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/grpcutil"
)

// ManagementServerParameters holds the gRPC and utility configuration
type ManagementServerParameters struct {
	Name             string `kong:"help='Cluster name',default='clusterfunk',short='n'"`
	Zeroconf         bool   `kong:"help='Use zeroconf discovery for Serf',default='true',short='z'"`
	Endpoint         string `kong:"help='gRPC management endpoint',short='e'"`
	TLS              bool   `kong:"help='TLS enabled for gRPC',short='T'"`
	CertFile         string `kong:"help='Client certificate for management service',type='existingfile',short='C'"`
	HostnameOverride string `kong:"help='Host name override for certificate',short='H'"`
}

// ConnectToManagement is a convenience method to get a cluster management
// client directly. If the zeroConf parameter is set to true ZeroConf will be
// used to locate a cluster node. The clusterName parameter is optional
// if the client parameters are set.
func ConnectToManagement(params ManagementServerParameters) (managepb.ClusterManagementClient, error) {
	if params.Zeroconf {
		if params.Name == "" {
			return nil, errors.New("needs a cluster name if zeroconf is to be used for discovery")
		}
		zr := toolbox.NewZeroconfRegistry(params.Name)
		ep, err := zr.ResolveFirst(funk.ZeroconfManagementKind, 1*time.Second)
		if err != nil {
			return nil, fmt.Errorf("zeroconf lookup error when searching for cluster %s: %v", params.Name, err)
		}
		params.Endpoint = ep
	}

	if params.Endpoint == "" {
		return nil, errors.New("need an endpoint for one of the cluster nodes")
	}

	grpcParams := grpcutil.GRPCClientParam{
		ServerEndpoint:     params.Endpoint,
		TLS:                params.TLS,
		CAFile:             params.CertFile,
		ServerHostOverride: params.HostnameOverride,
	}
	conn, err := grpcutil.NewGRPCClientConnection(grpcParams)
	if err != nil {
		return nil, fmt.Errorf("could not dial management endpoint for cluster %s. Is is it available? : %v", params.Name, err)

	}
	return managepb.NewClusterManagementClient(conn), nil
}

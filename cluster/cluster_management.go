package cluster

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/stalehd/clusterfunk/cluster/clustermgmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// This is the cluster management service implementations

// getGRPCOpts returns gRPC server options for the configuration
func (cf *clusterfunkCluster) getGRPCOpts(config GRPCServerParameters) ([]grpc.ServerOption, error) {
	if !config.TLS {
		return []grpc.ServerOption{}, nil
	}
	if config.CertFile == "" || config.KeyFile == "" {
		return nil, errors.New("missing cert file and key file parameters for GRPC server")
	}
	creds, err := credentials.NewServerTLSFromFile(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, err
	}
	return []grpc.ServerOption{grpc.Creds(creds)}, nil
}

func (cf *clusterfunkCluster) startManagementServices() error {
	opts, err := cf.getGRPCOpts(cf.config.Management)
	if err != nil {
		return err
	}
	cf.mgmtServer = grpc.NewServer(opts...)

	clustermgmt.RegisterClusterManagementServer(cf.mgmtServer, cf)

	listener, err := net.Listen("tcp", cf.config.Management.Endpoint)
	if err != nil {
		return err
	}

	if err := cf.mgmtServer.Serve(listener); err != nil {
		log.Printf("Unable to launch node management gRPC server: %v", err)
		return err
	}

	return nil
}

// Node management implementation
// -----------------------------------------------------------------------------

func (cf *clusterfunkCluster) GetState(context.Context, *clustermgmt.GetStateRequest) (*clustermgmt.GetStateResponse, error) {
	return nil, errors.New("not implemented")
}
func (cf *clusterfunkCluster) ListSerfNodes(context.Context, *clustermgmt.ListSerfNodesRequest) (*clustermgmt.ListSerfNodesResponse, error) {
	return nil, errors.New("not implemented")
}

// Leader management implementation, ie all Raft-related functions not covered by the node management implementation
// -----------------------------------------------------------------------------
func (cf *clusterfunkCluster) ListRaftNodes(context.Context, *clustermgmt.ListRaftNodesRequest) (*clustermgmt.ListRaftNodesResponse, error) {
	return nil, errors.New("not implemented")
}

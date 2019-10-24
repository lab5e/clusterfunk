package funk

import (
	"context"
	"errors"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/funk/clustermgmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// This is the cluster management service implementations

// getGRPCOpts returns gRPC server options for the configuration
func (c *clusterfunkCluster) getGRPCOpts(config GRPCServerParameters) ([]grpc.ServerOption, error) {
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

func (c *clusterfunkCluster) startManagementServices() error {
	opts, err := c.getGRPCOpts(c.config.Management)
	if err != nil {
		return err
	}
	c.mgmtServer = grpc.NewServer(opts...)

	clustermgmt.RegisterClusterManagementServer(c.mgmtServer, c)

	listener, err := net.Listen("tcp", c.config.Management.Endpoint)
	if err != nil {
		return err
	}

	fail := make(chan error)
	go func(ch chan error) {
		if err := c.mgmtServer.Serve(listener); err != nil {
			log.WithError(err).Error("Unable to launch node management gRPC interface")
			ch <- err
		}
	}(fail)

	select {
	case err := <-fail:
		return err
	case <-time.After(250 * time.Millisecond):
		// ok
	}
	c.SetEndpoint(ManagementEndpoint, listener.Addr().String())
	return nil
}

// Node management implementation
// -----------------------------------------------------------------------------

func (c *clusterfunkCluster) GetState(context.Context, *clustermgmt.GetStateRequest) (*clustermgmt.GetStateResponse, error) {
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) ListNodes(context.Context, *clustermgmt.ListNodesRequest) (*clustermgmt.ListNodesResponse, error) {
	if c.State() != Operational {
		return &clustermgmt.ListNodesResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) FindEndpoint(context.Context, *clustermgmt.EndpointRequest) (*clustermgmt.EndpointResponse, error) {
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) AddNode(context.Context, *clustermgmt.AddNodeRequest) (*clustermgmt.AddNodeResponse, error) {
	if c.State() != Operational {
		return &clustermgmt.AddNodeResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if c.Role() == Leader {
		// add the node
		return nil, errors.New("not implemented")
	}
	// proxy to the leader
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) RemoveNode(context.Context, *clustermgmt.RemoveNodeRequest) (*clustermgmt.RemoveNodeResponse, error) {
	if c.State() != Operational {
		return &clustermgmt.RemoveNodeResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if c.Role() == Leader {
		// add the node
		return nil, errors.New("not implemented")
	}
	// proxy to the leader
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) ListShards(ctx context.Context, req *clustermgmt.ListShardsRequest) (*clustermgmt.ListShardsResponse, error) {
	if c.State() != Operational {
		return &clustermgmt.ListShardsResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	return nil, errors.New("not implemented")
}

func (c *clusterfunkCluster) StepDown(ctx context.Context, req *clustermgmt.StepDownRequest) (*clustermgmt.StepDownResponse, error) {
	if c.State() != Operational {
		return &clustermgmt.StepDownResponse{
			Error: &clustermgmt.Error{
				ErrorCode: clustermgmt.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	return nil, errors.New("not implemented")
}

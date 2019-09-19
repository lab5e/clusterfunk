package cluster

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"time"

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

	fail := make(chan error)
	go func(ch chan error) {
		if err := cf.mgmtServer.Serve(listener); err != nil {
			log.Printf("Unable to launch node management gRPC server: %v", err)
			ch <- err
		}
	}(fail)

	select {
	case err := <-fail:
		return err
	case <-time.After(250 * time.Millisecond):
		// ok
	}
	cf.AddLocalEndpoint(ManagementEndpoint, listener.Addr().String())
	return nil
}

// Node management implementation
// -----------------------------------------------------------------------------

func (cf *clusterfunkCluster) GetState(context.Context, *clustermgmt.GetStateRequest) (*clustermgmt.GetStateResponse, error) {
	ret := &clustermgmt.GetStateResponse{
		NodeId:    cf.config.NodeID,
		RaftState: cf.raftNode.State(),
	}

	ret.RaftNodeCount = int32(cf.raftNode.MemberCount())
	ret.SerfNodeCount = int32(cf.serfNode.MemberCount())
	return ret, nil
}

func (cf *clusterfunkCluster) ListSerfNodes(context.Context, *clustermgmt.ListSerfNodesRequest) (*clustermgmt.ListSerfNodesResponse, error) {

	ret := &clustermgmt.ListSerfNodesResponse{
		NodeId: cf.config.NodeID,
	}
	members := cf.serfNode.Members()
	ret.Swarm = make([]*clustermgmt.SerfNodeInfo, len(members))

	for i, v := range members {
		ret.Swarm[i] = &clustermgmt.SerfNodeInfo{
			Id:       v.NodeID,
			Endpoint: v.Tags[SerfEndpoint],
			Status:   v.Status,
		}
		for k, v := range v.Tags {
			if strings.HasPrefix(k, clusterEndpointPrefix) {
				ret.Swarm[i].ServiceEndpoints = append(ret.Swarm[i].ServiceEndpoints, &clustermgmt.SerfEndpoint{Name: k, HostPort: v})
				continue
			}
			ret.Swarm[i].Attributes = append(ret.Swarm[i].Attributes, &clustermgmt.ServiceAttribute{Name: k, Value: v})
		}
	}
	return ret, nil
}

// Leader management implementation, ie all Raft-related functions not covered by the node management implementation
// -----------------------------------------------------------------------------
func (cf *clusterfunkCluster) ListRaftNodes(context.Context, *clustermgmt.ListRaftNodesRequest) (*clustermgmt.ListRaftNodesResponse, error) {

	ret := &clustermgmt.ListRaftNodesResponse{
		NodeId: cf.config.NodeID,
	}

	list, err := cf.raftNode.MemberList()
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		ret.Members = append(ret.Members, &clustermgmt.RaftNodeInfo{
			Id:        v.ID,
			RaftState: v.State,
			IsLeader:  v.Leader,
		})
	}
	return ret, nil
}

package cluster

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"net"
	"sync/atomic"
	"time"

	"github.com/stalehd/clusterfunk/cluster/clusterproto"
	"google.golang.org/grpc"
)

func (c *clusterfunkCluster) startLeaderService() error {
	// TODO: use common configuration for internal RPC
	leaderConfig := GRPCServerParameters{
		Endpoint: c.config.LeaderEndpoint,
		TLS:      false,
		CertFile: "",
		KeyFile:  "",
	}

	opts, err := c.getGRPCOpts(leaderConfig)
	if err != nil {
		return err
	}
	c.leaderServer = grpc.NewServer(opts...)
	clusterproto.RegisterClusterLeaderServiceServer(c.leaderServer, c)

	listener, err := net.Listen("tcp", leaderConfig.Endpoint)
	if err != nil {
		return err
	}

	fail := make(chan error)
	go func(ch chan error) {
		if err := c.leaderServer.Serve(listener); err != nil {
			log.WithError(err).Error("Unable to launch leader gRPC interface")
			ch <- err
		}
	}(fail)

	select {
	case err := <-fail:
		return err
	case <-time.After(250 * time.Millisecond):
		// ok
	}
	c.AddLocalEndpoint(LeaderEndpoint, listener.Addr().String())
	return nil
}

func (c *clusterfunkCluster) ConfirmShardMap(ctx context.Context, req *clusterproto.ConfirmShardMapRequest) (*clusterproto.ConfirmShardMapResponse, error) {
	// Ensure we're the leader and we're resharding the cluster
	if c.Role() != Leader {
		return nil, errors.New("not a leader")
	}
	if c.LocalState() != Resharding {
		return nil, errors.New("not in resharding mode")
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()
	reshardIndex := atomic.LoadUint64(c.reshardingLogIndex)
	if req.LogIndex < int64(reshardIndex) {
		// This is not the ack we're looking for
		return &clusterproto.ConfirmShardMapResponse{
			Success:      false,
			CurrentIndex: int64(reshardIndex),
		}, nil
	}
	if req.LogIndex != int64(reshardIndex) {
		return nil, errors.New("index is invalid")
	}
	c.setFSMState(ackReceived, req.NodeID)
	return &clusterproto.ConfirmShardMapResponse{
		Success:      true,
		CurrentIndex: int64(reshardIndex),
	}, nil
}

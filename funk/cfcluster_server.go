package funk

import (
	"context"
	"errors"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/funk/clusterproto"
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
	return nil
}

func (c *clusterfunkCluster) ConfirmShardMap(ctx context.Context, req *clusterproto.ConfirmShardMapRequest) (*clusterproto.ConfirmShardMapResponse, error) {
	// Ensure we're the leader and we're resharding the cluster
	if c.Role() != Leader {
		return nil, errors.New("not a leader")
	}
	if c.State() != Resharding {
		log.WithFields(log.Fields{
			"state": c.State().String(),
			"index": req.LogIndex,
			"node":  req.NodeID,
		}).Warning("not in resharding mode")
		return nil, errors.New("not in resharding mode")
	}

	logIndex := c.ProposedShardMapIndex()
	if uint64(req.LogIndex) != logIndex {
		// This is not the ack we're looking for
		return &clusterproto.ConfirmShardMapResponse{
			Success:      false,
			CurrentIndex: int64(logIndex),
		}, nil
	}

	if c.handleAckReceived(req.NodeID, uint64(req.LogIndex)) {
		return &clusterproto.ConfirmShardMapResponse{
			Success:      true,
			CurrentIndex: int64(logIndex),
		}, nil
	}
	return &clusterproto.ConfirmShardMapResponse{
		Success:      false,
		CurrentIndex: int64(logIndex),
	}, nil
}

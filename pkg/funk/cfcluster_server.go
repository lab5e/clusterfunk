package funk

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"context"
	"errors"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/funk/clusterpb"
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
	clusterpb.RegisterClusterLeaderServiceServer(c.leaderServer, c)

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

func (c *clusterfunkCluster) ConfirmShardMap(ctx context.Context, req *clusterpb.ConfirmShardMapRequest) (*clusterpb.ConfirmShardMapResponse, error) {
	// Ensure we're the leader and we're resharding the cluster
	if c.Role() != Leader {
		return nil, errors.New("not a leader")
	}
	if c.State() != Resharding {
		log.WithFields(log.Fields{
			"state": c.State().String(),
			"index": req.LogIndex,
			"node":  req.NodeId,
		}).Warning("not in resharding mode")
		return nil, errors.New("not in resharding mode")
	}

	if uint64(req.LogIndex) != c.unacknowledged.ShardIndex() {
		// This is not the ack we're looking for
		return &clusterpb.ConfirmShardMapResponse{
			Success:      false,
			CurrentIndex: int64(c.unacknowledged.ShardIndex()),
		}, nil
	}

	if c.handleAckReceived(req.NodeId, uint64(req.LogIndex)) {
		return &clusterpb.ConfirmShardMapResponse{
			Success:      true,
			CurrentIndex: req.LogIndex,
		}, nil
	}

	return &clusterpb.ConfirmShardMapResponse{
		Success:      false,
		CurrentIndex: int64(c.unacknowledged.ShardIndex()),
	}, nil
}

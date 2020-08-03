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
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/clusterfunk/pkg/toolbox"

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

	managepb.RegisterClusterManagementServer(c.mgmtServer, c)

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

func (c *clusterfunkCluster) clearLeaderManagementClient() {
	c.leaderClientMutex.Lock()
	defer c.leaderClientMutex.Unlock()
	if c.leaderClientConn != nil {
		c.leaderClientConn.Close()
		c.leaderClientConn = nil
	}
	c.leaderClient = nil
}

// Node management implementation
// -----------------------------------------------------------------------------
func (c *clusterfunkCluster) leaderManagementClient() (managepb.ClusterManagementClient, error) {
	if c.leaderClient == nil {
		ep := c.GetEndpoint(c.raftNode.LeaderNodeID(), ManagementEndpoint)

		// TODO: Custom gRPC parameters goes here. Set cert if required
		opts, err := toolbox.GetGRPCDialOpts(toolbox.GRPCClientParam{})
		if err != nil {
			return nil, err
		}
		c.leaderClientMutex.Lock()
		defer c.leaderClientMutex.Unlock()
		c.leaderClientConn, err = grpc.Dial(ep, opts...)
		if err != nil {
			return nil, err
		}
		c.leaderClient = managepb.NewClusterManagementClient(c.leaderClientConn)
	}
	return c.leaderClient, nil
}

func (c *clusterfunkCluster) GetStatus(ctx context.Context, req *managepb.GetStatusRequest) (*managepb.GetStatusResponse, error) {
	var ret *managepb.GetStatusResponse

	switch c.State() {
	case Invalid, Stopping, Starting, Joining:
		ret = &managepb.GetStatusResponse{
			ClusterName: c.Name(),
			LocalState:  c.State().String(),
			LocalRole:   c.Role().String(),
			Error: &managepb.Error{
				ErrorCode: managepb.Error_INVALID,
				Message:   fmt.Sprintf("Not in a cluster. State is %s", c.State().String()),
			},
		}

	case Operational, Voting, Resharding:
		ret = &managepb.GetStatusResponse{
			ClusterName:   c.Name(),
			LocalNodeId:   c.NodeID(),
			LocalState:    c.State().String(),
			LocalRole:     c.Role().String(),
			RaftNodeCount: 0,
			SerfNodeCount: 0,
			LeaderNodeId:  "",
		}

		if c.raftNode.Leader() {
			ret.RaftNodeCount = int32(c.raftNode.Nodes.Size())
			ret.SerfNodeCount = int32(c.serfNode.Size())
			ret.LeaderNodeId = c.NodeID()
			ret.ShardCount = int32(c.shardManager.ShardCount())
			ret.ShardWeight = int32(c.shardManager.TotalWeight())
		} else {
			if c.State() != Voting {
				leader, err := c.leaderManagementClient()
				if err != nil {
					return nil, err
				}
				leaderRet, err := leader.GetStatus(ctx, req)
				if err != nil {
					return nil, err
				}
				ret.RaftNodeCount = leaderRet.RaftNodeCount
				ret.SerfNodeCount = leaderRet.SerfNodeCount
				ret.LeaderNodeId = leaderRet.LocalNodeId
				ret.ShardCount = leaderRet.ShardCount
				ret.ShardWeight = leaderRet.ShardWeight
			}
		}
	}
	return ret, nil
}

func (c *clusterfunkCluster) ListNodes(ctx context.Context, req *managepb.ListNodesRequest) (*managepb.ListNodesResponse, error) {
	if c.State() != Operational {
		return &managepb.ListNodesResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if !c.raftNode.Leader() {
		client, err := c.leaderManagementClient()
		if err != nil {
			return nil, err
		}
		ret, err := client.ListNodes(ctx, req)
		if err != nil {
			return nil, err
		}
		ret.NodeId = c.NodeID()
		return ret, nil
	}

	nodes := make(map[string]*managepb.NodeInfo)
	ret := &managepb.ListNodesResponse{
		LeaderId: c.NodeID(),
		NodeId:   c.NodeID(),
		Nodes:    make([]*managepb.NodeInfo, 0),
	}

	list, err := c.raftNode.memberList()
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		nodes[v.ID] = &managepb.NodeInfo{
			NodeId:    v.ID,
			RaftState: v.State,
			Leader:    v.Leader,
		}
	}

	for _, v := range c.serfNode.memberList() {
		n, ok := nodes[v.ID]
		if !ok {
			n = &managepb.NodeInfo{
				NodeId:    v.ID,
				RaftState: "",
				Leader:    false,
			}
		}
		n.SerfState = v.State
		nodes[v.ID] = n
	}

	for _, v := range nodes {
		ret.Nodes = append(ret.Nodes, v)
	}
	return ret, nil
}

func (c *clusterfunkCluster) FindEndpoint(ctx context.Context, req *managepb.EndpointRequest) (*managepb.EndpointResponse, error) {
	ret := &managepb.EndpointResponse{
		NodeId:    c.NodeID(),
		Endpoints: make([]*managepb.EndpointInfo, 0),
	}
	for _, v := range c.serfNode.Nodes() {
		for k, val := range v.Tags {
			if strings.HasPrefix(k, EndpointPrefix) {
				if strings.Contains(k, req.EndpointName) {
					ret.Endpoints = append(ret.Endpoints, &managepb.EndpointInfo{
						NodeId:   v.NodeID,
						Name:     k,
						HostPort: val,
					})
				}
			}
		}
	}
	return ret, nil
}

func (c *clusterfunkCluster) ListEndpoints(ctx context.Context, req *managepb.ListEndpointRequest) (*managepb.ListEndpointResponse, error) {
	ret := &managepb.ListEndpointResponse{
		NodeId:    c.NodeID(),
		Endpoints: make([]*managepb.EndpointInfo, 0),
	}
	for _, v := range c.serfNode.Nodes() {
		for k, val := range v.Tags {
			if strings.HasPrefix(k, EndpointPrefix) {
				ret.Endpoints = append(ret.Endpoints, &managepb.EndpointInfo{
					NodeId:   v.NodeID,
					Name:     k,
					HostPort: val,
				})
			}
		}
	}
	return ret, nil
}

func (c *clusterfunkCluster) AddNode(ctx context.Context, req *managepb.AddNodeRequest) (*managepb.AddNodeResponse, error) {
	if c.State() != Operational {
		return &managepb.AddNodeResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if c.Role() == Leader {
		ret := &managepb.AddNodeResponse{
			NodeId: c.NodeID(),
		}
		if c.raftNode.Nodes.Contains(req.NodeId) {
			ret.Error = &managepb.Error{
				ErrorCode: managepb.Error_INVALID,
				Message:   "Node is already a member of the cluster",
			}
			return ret, nil
		}
		ep := c.GetEndpoint(req.NodeId, RaftEndpoint)
		if ep == "" {
			ret.Error = &managepb.Error{
				ErrorCode: managepb.Error_UNKNOWN_ID,
				Message:   "Unknown node",
			}
			return ret, nil
		}
		if err := c.raftNode.AddClusterNode(req.NodeId, ep); err != nil {
			ret.Error = &managepb.Error{
				ErrorCode: managepb.Error_GENERIC,
				Message:   err.Error(),
			}
		}
		// add the node
		return ret, nil
	}
	leader, err := c.leaderManagementClient()
	if err != nil {
		return nil, err
	}
	return leader.AddNode(ctx, req)
}

func (c *clusterfunkCluster) RemoveNode(ctx context.Context, req *managepb.RemoveNodeRequest) (*managepb.RemoveNodeResponse, error) {
	if c.State() != Operational {
		return &managepb.RemoveNodeResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if c.Role() == Leader {
		ret := &managepb.RemoveNodeResponse{
			NodeId: c.NodeID(),
		}
		ep := c.GetEndpoint(req.NodeId, RaftEndpoint)
		if ep == "" || !c.raftNode.Nodes.Contains(req.NodeId) {
			ret.Error = &managepb.Error{
				ErrorCode: managepb.Error_UNKNOWN_ID,
				Message:   "Node is not a member of the Serf cluster",
			}
			return ret, nil
		}
		if err := c.raftNode.RemoveClusterNode(req.NodeId, ep); err != nil {
			ret.Error = &managepb.Error{
				ErrorCode: managepb.Error_GENERIC,
				Message:   err.Error(),
			}
		}
		// add the node
		return ret, nil
	}
	leader, err := c.leaderManagementClient()
	if err != nil {
		return nil, err
	}
	return leader.RemoveNode(ctx, req)
}

func (c *clusterfunkCluster) ListShards(ctx context.Context, req *managepb.ListShardsRequest) (*managepb.ListShardsResponse, error) {
	if c.State() != Operational {
		return &managepb.ListShardsResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	items := make(map[string]*managepb.ShardInfo)

	ret := &managepb.ListShardsResponse{
		NodeId: c.NodeID(),
		Shards: make([]*managepb.ShardInfo, 0),
	}
	shards := c.shardManager.Shards()
	ret.TotalShards = int32(len(shards))
	ret.TotalWeight = int32(c.shardManager.TotalWeight())
	for _, v := range shards {
		i := items[v.NodeID()]
		if i == nil {
			i = &managepb.ShardInfo{
				NodeId:      v.NodeID(),
				ShardCount:  0,
				ShardWeight: 0,
			}
		}
		i.ShardCount++
		i.ShardWeight += int32(v.Weight())
		items[v.NodeID()] = i
	}

	for _, v := range items {
		ret.Shards = append(ret.Shards, v)
	}
	return ret, nil
}

func (c *clusterfunkCluster) StepDown(ctx context.Context, req *managepb.StepDownRequest) (*managepb.StepDownResponse, error) {
	if c.State() != Operational {
		return &managepb.StepDownResponse{
			Error: &managepb.Error{
				ErrorCode: managepb.Error_NO_LEADER,
				Message:   "Cluster is not in operational state",
			},
		}, nil
	}
	if c.raftNode.Leader() {
		if err := c.raftNode.StepDown(); err != nil {
			return &managepb.StepDownResponse{
				Error: &managepb.Error{
					ErrorCode: managepb.Error_GENERIC,
					Message:   err.Error(),
				},
			}, nil
		}
		return &managepb.StepDownResponse{
			NodeId: c.NodeID(),
		}, nil
	}

	client, err := c.leaderManagementClient()
	if err != nil {
		return nil, err
	}
	return client.StepDown(ctx, req)
}

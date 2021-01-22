package http

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
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
)

// This file contains the websocket events that will be sent to the client
// from the web server. It's mostly just reformatting of data.

type clusterStatus struct {
	Type   string `json:"type"`
	NodeID string `json:"nodeId"`
	State  string `json:"state"`
	Role   string `json:"role"`
}

func newClusterStatus(cluster funk.Cluster) clusterStatus {
	return clusterStatus{
		Type:   "status",
		NodeID: cluster.NodeID(),
		State:  cluster.State().String(),
		Role:   cluster.Role().String(),
	}
}

type shardMap struct {
	Type   string         `json:"type"`
	Shards map[string]int `json:"shards"`
}

func newShardMap(shardMapper sharding.ShardMap) shardMap {
	m := make(map[string]int)
	for _, v := range shardMapper.Shards() {
		n := m[v.NodeID()]
		n++
		m[v.NodeID()] = n
	}
	return shardMap{
		Type:   "shards",
		Shards: m,
	}
}

type memberNode struct {
	ID              string `json:"id"`
	WebEndpoint     string `json:"http"`
	MetricsEndpoint string `json:"metrics"`
}

type memberList struct {
	Type    string       `json:"type"`
	Members []memberNode `json:"members"`
	Leader  string       `json:"leaderId"`
}

func newMemberList(cluster funk.Cluster) memberList {
	ret := memberList{
		Type:    "members",
		Members: make([]memberNode, 0),
		Leader:  cluster.Leader(),
	}
	for _, v := range cluster.Nodes() {
		ret.Members = append(ret.Members, memberNode{
			ID:              v,
			WebEndpoint:     cluster.GetEndpoint(v, ConsoleEndpointName),
			MetricsEndpoint: cluster.GetEndpoint(v, MetricsEndpointName),
		})
	}
	return ret
}

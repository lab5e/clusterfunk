package http

import (
	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/funk/sharding"
)

// This file contains the websocket events that will be sent to the client
// from the web server. It's mostly just reformatting of data.

type clusterStatus struct {
	Type  string `json:"type"`
	State string `json:"state"`
	Role  string `json:"role"`
}

func newClusterStatus(cluster funk.Cluster) clusterStatus {
	return clusterStatus{
		Type:  "status",
		State: cluster.State().String(),
		Role:  cluster.Role().String(),
	}
}

type shardMap struct {
	Type   string         `json:"type"`
	Shards map[string]int `json:"shards"`
}

func newShardMap(shardManager sharding.ShardManager) shardMap {
	m := make(map[string]int)
	for _, v := range shardManager.Shards() {
		n := m[v.NodeID()]
		n += v.Weight()
		m[v.NodeID()] = n
	}
	return shardMap{
		Type:   "shards",
		Shards: m,
	}
}

type memberNode struct {
	ID          string `json:"id"`
	WebEndpoint string `json:"http"`
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
			ID:          v,
			WebEndpoint: cluster.GetEndpoint(v, ConsoleEndpoint),
		})
	}
	return ret
}

type nodeInfo struct {
	Type   string `json:"type"`
	NodeID string `json:"nodeId"`
	State  string `json:"state"`
	Role   string `json:"role"`
}

func newNodeInfo(cluster funk.Cluster) nodeInfo {
	return nodeInfo{
		Type:   "info",
		NodeID: cluster.NodeID(),
		State:  cluster.State().String(),
		Role:   cluster.Role().String(),
	}
}

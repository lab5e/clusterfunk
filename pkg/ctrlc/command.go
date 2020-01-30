package ctrlc

import "errors"

// Command is the main parameter struct for the ctrlc utility
type Command struct {
	ClusterName      string           `kong:"help='Cluster name',default='clusterfunk',short='n'"`
	Zeroconf         bool             `kong:"help='Use zeroconf discovery for Serf',default='true',short='z'"`
	Endpoint         string           `kong:"help='gRPC management endpoint',short='e'"`
	TLS              bool             `kong:"help='TLS enabled for gRPC',short='T'"`
	CertFile         string           `kong:"help='Client certificate for management service',type='path',short='C'"`
	HostnameOverride string           `kong:"help='Host name override for certificate',short='H'"`
	Status           StatusCommand    `kong:"cmd,help='Show the node status'"`
	Nodes            ListNodesCommand `kong:"cmd,help='List the nodes in the cluster'"`
	Endpoints        EndpointsCommand `kong:"cmd,help='List endpoints known by the node'"`
	Node             NodeCommand      `kong:"cmd,help='Add and remove nodes in cluster'"`
	Shards           ShardsCommand    `kong:"cmd,help='Show the shards in the cluster'"`
	StepDown         StepDownCommand  `kong:"cmd,help='Step down as the current leader'"`
}

// We won't be using the errors returned from the commands in Kong so this is
// a placeholder error that we'll return on errors
var errStd = errors.New("error")

/*
func (c *removeNodeCommand) Run(param *Command) error {
	client := connectToManagement(param)
	if client == nil {
		return errStd
	}

	return nil
}

*/

package ctrlc

import (
	"errors"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
)

// CommandList contains all of the commands for the ctrlc utility
type CommandList struct {
	Status    StatusCommand      `kong:"cmd,help='Show the node status'"`
	Nodes     ListNodesCommand   `kong:"cmd,help='List the nodes in the cluster'"`
	Endpoints EndpointsCommand   `kong:"cmd,help='List endpoints known by the node'"`
	Node      NodeCommand        `kong:"cmd,help='Add and remove nodes in cluster'"`
	Shards    ShardsCommand      `kong:"cmd,help='Show the shards in the cluster'"`
	StepDown  StepDownCommand    `kong:"cmd,help='Step down as the current leader'"`
	Diag      DiagnosticsCommand `kong:"cmd,help='Show diagnostic information for zeroconf, serf and raft'"`
}

// Parameters is the main parameter struct for the ctrlc utility
type Parameters struct {
	Server   clientfunk.ClientParameters `kong:"embed"`
	Commands CommandList                 `kong:"embed"`
}

// ClientParams returns the cluster client parameters
func (p *Parameters) ClientParams() clientfunk.ClientParameters {
	return p.Server
}

// ClusterCommands returns the list of commands for the management utility
func (p *Parameters) ClusterCommands() CommandList {
	return p.Commands
}

// We won't be using the errors returned from the commands in Kong so this is
// a placeholder error that we'll return on errors
var errStd = errors.New("error")

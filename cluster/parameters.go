package cluster

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/stalehd/clusterfunk/utils"
)

// GRPCServerParameters is a parameter struct for gRPC services
type GRPCServerParameters struct {
	Endpoint string `param:"desc=Server endpoint;default="`
	TLS      bool   `param:"desc=Enable TLS;default=false"`
	CertFile string `param:"desc=Certificate file;file"`
	KeyFile  string `param:"desc=Certificate key file;file"`
}

// Parameters is the parameters required for the cluster. The defaults are
// suitable for a development cluster but not for a production cluster.
type Parameters struct {
	Raft         RaftParameters
	AutoJoin     bool   `param:"desc=Auto join via SerfEvents;default=true"`
	ClusterName  string `param:"desc=Cluster name;default=clusterfunk"`
	Loopback     bool   `param:"desc=Use loopback adapter;default=false"`
	Join         string `param:"desc=Join address and port for Serf cluster"`
	Interface    string `param:"desc=Interface address for services"`
	SerfEndpoint string `param:"desc=Endpoint for Serf;default="`
	Verbose      bool   `param:"desc=Verbose logging for Serf and Raft;default=false"`
	NodeID       string `param:"desc=Node ID for Serf and Raft;default="`
	ZeroConf     bool   `param:"desc=Zero-conf startup;default=true"`
	Management   GRPCServerParameters
	NonVoting    bool `param:"desc=Nonvoting node;default=false"`
}

func (p *Parameters) checkAndSetEndpoint(hostport *string) {
	if *hostport != "" {
		return
	}
	port, err := utils.FreeTCPPort()
	if err != nil {
		port = int(rand.Int31n(31000) + 1024)
	}
	*hostport = fmt.Sprintf("%s:%d", p.Interface, port)
}
func (p *Parameters) final() {
	if p.NodeID == "" {
		p.NodeID = utils.RandomID()
	}
	if p.Interface == "" {
		ip, err := utils.FindPublicIPv4()
		p.Interface = ip.String()
		if err != nil {
			log.Printf("Unable to get public IP: %v", err)
			p.Interface = "localhost"
		}
		if p.Loopback {
			p.Interface = "127.0.0.1"
		}
	}
	p.checkAndSetEndpoint(&p.Raft.RaftEndpoint)
	p.checkAndSetEndpoint(&p.SerfEndpoint)
	p.checkAndSetEndpoint(&p.Management.Endpoint)

	if p.Verbose {
		log.Printf("Configuration: %+v", p)
	}
}

// NodeType returns the type of node this will be announced as.
func (p *Parameters) NodeType() string {
	if p.NonVoting {
		return NonvoterKind
	}
	return VoterKind
}

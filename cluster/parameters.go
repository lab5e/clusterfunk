package cluster

import (
	"fmt"
	"math/rand"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/toolbox"
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
	Raft           RaftParameters
	Serf           SerfParameters
	AutoJoin       bool   `param:"desc=Auto join via SerfEvents;default=true"`
	ClusterName    string `param:"desc=Cluster name;default=clusterfunk"`
	Interface      string `param:"desc=Interface address for services"`
	Verbose        bool   `param:"desc=Verbose logging for Serf and Raft;default=false"`
	NodeID         string `param:"desc=Node ID for Serf and Raft;default="`
	ZeroConf       bool   `param:"desc=Zero-conf startup;default=true"`
	Management     GRPCServerParameters
	NonVoting      bool   `param:"desc=Nonvoting node;default=false"`
	NonMember      bool   `param:"desc=Non-member;default=false"`
	LeaderEndpoint string // This isn't a parameter, it's set by the service
}

func (p *Parameters) checkAndSetEndpoint(hostport *string) {
	if *hostport != "" {
		return
	}
	port, err := toolbox.FreeTCPPort()
	if err != nil {
		port = int(rand.Int31n(31000) + 1024)
	}
	*hostport = fmt.Sprintf("%s:%d", p.Interface, port)
}

// Final sets the defaults for the parameters
func (p *Parameters) Final() {
	if p.NodeID == "" {
		p.NodeID = toolbox.RandomID()
	}
	if p.Interface == "" {
		ip, err := toolbox.FindPublicIPv4()
		p.Interface = ip.String()
		if err != nil {
			log.WithError(err).Error("Unable to get public IP")
			p.Interface = "localhost"
		}
	}
	p.checkAndSetEndpoint(&p.Raft.RaftEndpoint)
	p.checkAndSetEndpoint(&p.Serf.Endpoint)
	p.checkAndSetEndpoint(&p.Management.Endpoint)
	p.checkAndSetEndpoint(&p.LeaderEndpoint)

	if p.Verbose {
		log.Infof("Configuration: %+v", p)
	}
}

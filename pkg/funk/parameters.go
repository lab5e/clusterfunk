package funk

import (
	"fmt"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/stalehd/clusterfunk/pkg/toolbox"
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
// The struct uses annotations from https://github.com/ExploratoryEngineering/params
type Parameters struct {
	AutoJoin         bool          `param:"desc=Auto join via SerfEvents;default=true"`
	Name             string        `param:"desc=Cluster name;default=clusterfunk"`
	Interface        string        `param:"desc=Interface address for services"`
	Verbose          bool          `param:"desc=Verbose logging for Serf and Raft;default=false"`
	NodeID           string        `param:"desc=Node ID for Serf and Raft;default="`
	ZeroConf         bool          `param:"desc=Zero-conf startup;default=true"`
	NonVoting        bool          `param:"desc=Nonvoting node;default=false"`
	NonMember        bool          `param:"desc=Non-member;default=false"`
	LivenessInterval time.Duration `param:"desc=Liveness checker intervals;default=150ms"`
	LivenessRetries  int           `param:"desc=Number of retries for liveness checks;default=3"`
	LivenessEndpoint string        `param:"desc=Liveness UDP endpoint"`
	AckTimeout       time.Duration `param:"desc=Ack timeout for nodes in the cluster;default=500ms"`
	Metrics          string        `param:"desc=Metrics sink to use;options=blackhole,prometheus;default=prometheus"`
	Raft             RaftParameters
	Serf             SerfParameters
	LeaderEndpoint   string // This isn't a parameter, it's set by the service
	Management       GRPCServerParameters
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

// Final sets the defaults for the parameters that haven't got a sensible value,
// f.e. endpoints and defaults. Defaults that are random values can't be
// set via the parameter library. Yet.
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
	p.checkAndSetEndpoint(&p.LivenessEndpoint)

	// Log endpoints regardless of verbose or not.
	log.WithFields(log.Fields{
		"serfEndpoint":       p.Serf.Endpoint,
		"raftEndpoint":       p.Raft.RaftEndpoint,
		"managementEndpoint": p.Management.Endpoint,
		"livenessEndpoint":   p.LivenessEndpoint,
	}).Info("Endpoint configuration")
}

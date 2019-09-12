package cluster

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/stalehd/clattering/utils"
)

// Parameters is the parameters required for the cluster. The defaults are
// suitable for a development cluster but not for a production cluster.
type Parameters struct {
	ClusterName  string `param:"desc=Cluster name;default=clattering"`
	Loopback     bool   `param:"desc=Use loopback adapter;default=false"`
	Join         string `param:"desc=Join address and port for Serf cluster"`
	Interface    string `param:"desc=Interface address for services"`
	SerfEndpoint string `param:"desc=Endpoint for Serf;default="`
	RaftEndpoint string `param:"desc=Endpoint for Raft;default="`
	Verbose      bool   `param:"desc=Verbose logging for Serf and Raft;default=false"`
	NodeID       string `param:"desc=Node ID for Serf and Raft;default="`
	Bootstrap    bool   `param:"desc=Bootstrap a new Raft cluster;default=false"`
	DiskStore    bool   `param:"desc=Disk-based store;default=false"`
	ZeroConf     bool   `param:"desc=Zero-conf startup;default=true"`
}

func (p *Parameters) final() {
	if p.NodeID == "" {
		p.NodeID = randomID()
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
	if p.RaftEndpoint == "" {
		port, err := utils.FreeTCPPort()
		if err != nil {
			port = int((rand.Int31() + 1024) % 32000)
		}
		p.RaftEndpoint = fmt.Sprintf("%s:%d", p.Interface, port)
	}
	if p.SerfEndpoint == "" || p.SerfEndpoint == ":0" {
		port, err := utils.FreeTCPPort()
		if err != nil {
			port = int((rand.Int31() + 1024) % 32000)
		}
		p.SerfEndpoint = fmt.Sprintf("%s:%d", p.Interface, port)
	}
	if p.Verbose {
		log.Printf("Configuration: %+v", p)
	}
}

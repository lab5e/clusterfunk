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
	"fmt"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/netutils"
)

// GRPCServerParameters is a parameter struct for gRPC services
// The struct uses annotations from Kong (https://github.com/alecthomas/kong)
type GRPCServerParameters struct {
	Endpoint string `kong:"help='Server endpoint'"`
	TLS      bool   `kong:"help='Enable TLS'"`
	CertFile string `kong:"help='Certificate file',type='existingfile'"`
	KeyFile  string `kong:"help='Certificate key file',type='existingfile'"`
}

// Parameters is the parameters required for the cluster. The defaults are
// suitable for a development cluster but not for a production cluster.
type Parameters struct {
	AutoJoin         bool                 `kong:"help='Auto join via SerfEvents',default='true'"`
	Name             string               `kong:"help='Cluster name',default='clusterfunk'"`
	Interface        string               `kong:"help='Interface address for services'"`
	Verbose          bool                 `kong:"help='Verbose logging for Serf and Raft'"`
	NodeID           string               `kong:"help='Node ID for Serf and Raft'"`
	ZeroConf         bool                 `kong:"help='Zero-conf startup',default='true'"`
	NonVoting        bool                 `kong:"help='Nonvoting node',default='false'"`
	NonMember        bool                 `kong:"help='Non-member',default='false'"`
	LivenessInterval time.Duration        `kong:"help='Liveness checker intervals',default='150ms'"`
	LivenessRetries  int                  `kong:"help='Number of retries for liveness checks',default='3'"`
	LivenessEndpoint string               `kong:"help='Liveness UDP endpoint'"`
	AckTimeout       time.Duration        `kong:"help='Ack timeout for nodes in the cluster',default='500ms'"`
	Metrics          string               `kong:"help='Metrics sink to use',enum='blackhole,prometheus',default='prometheus'"`
	Raft             RaftParameters       `kong:"embed,prefix='raft-'"`
	Serf             SerfParameters       `kong:"embed,prefix='serf-'"`
	Management       GRPCServerParameters `kong:"embed,prefix='management-'"`
	LeaderEndpoint   string               // This isn't a parameter, it's set by the service
}

func (p *Parameters) checkAndSetEndpoint(hostport *string) {
	if *hostport != "" {
		return
	}
	port, err := netutils.FreeTCPPort()
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
	p.Serf.Final()
	p.checkAndSetEndpoint(&p.Raft.RaftEndpoint)
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

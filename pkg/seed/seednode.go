package seed

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/netutils"
	gotoolbox "github.com/lab5e/gotoolbox/toolbox"
	"github.com/sirupsen/logrus"
)

// TODO: (stalehd): Use client parameters here.

type parameters struct {
	ClusterName      string                  `kong:"help='Cluster name',default='clusterfunk'"`
	ZeroConf         bool                    `kong:"help='ZeroConf lookups for cluster',default='true'"`
	NodeID           string                  `kong:"help='Node ID for seed node',default=''"`
	SerfPort         int                     `kong:"help='Port to use for Serf',default='0'"`
	SerfJoinAddress  []string                `kong:"help='Join address for Serf', default=''"`
	Log              gotoolbox.LogParameters `kong:"embed,prefix='log-'"`
	LiveView         bool                    `kong:"help='Display live view of nodes',default='false'"`
	ShowAllNodes     bool                    `kong:"help='Show all nodes, not just nodes alive',default='false'"`
	SerfSnapshotPath string                  `kong:"help='Serf snapshot path (for persistence)'"`
	HTTP             bool                    `kong:"help='Launch embedded web server with a status page',default='false'"`
}

// Run is a ready-to run (just call it from main()) implementation of
// a simple seed node.
func Run() {
	var config parameters
	k, err := kong.New(&config, kong.Name("seed"),
		kong.Description("Seed node demo"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: false,
		}))
	if err != nil {
		panic(err)
	}
	if _, err := k.Parse(os.Args[1:]); err != nil {
		k.FatalIfErrorf(err)
		return
	}

	gotoolbox.InitLogs("seed", config.Log)
	if config.NodeID == "" {
		config.NodeID = toolbox.RandomID()
	}
	if config.SerfPort == 0 {
		config.SerfPort, err = netutils.FreeTCPPort()
		if err != nil {
			logrus.WithError(err).Error("Could not find free port")
			os.Exit(2)
		}
	}
	publicIP, err := netutils.FindPublicIPv4()
	if err != nil {
		logrus.WithError(err).Error("Could not determine public IP")
		os.Exit(2)
	}
	serfConfig := funk.SerfParameters{
		Endpoint:     fmt.Sprintf("%s:%d", publicIP, config.SerfPort),
		Verbose:      (logrus.GetLevel() >= logrus.DebugLevel),
		JoinAddress:  config.SerfJoinAddress,
		SnapshotPath: config.SerfSnapshotPath,
	}
	serfConfig.Final()

	logrus.WithFields(logrus.Fields{
		"nodeID":       config.NodeID,
		"serfEndpoint": serfConfig.Endpoint,
		"joinAddress":  serfConfig.JoinAddress,
	}).Info("Serf configuration")

	logrus.WithField("nodeId", config.NodeID).Info("Starting seed node")

	if config.ZeroConf {
		zr := toolbox.NewZeroconfRegistry(config.ClusterName)

		if err := zr.Register(funk.ZeroconfSerfKind, config.NodeID, config.SerfPort); err != nil {
			logrus.WithError(err).Error("Unable to register in ZeroConf")
			os.Exit(2)
		}
		defer zr.Shutdown()

		// Check if there's other nodes we can join here.
		logrus.Info("Looking for other nodes in the cluster")
		nodes, err := zr.Resolve(funk.ZeroconfSerfKind, time.Second*1)
		if err != nil {
			logrus.WithError(err).Error("Could not do zeroconf lookup")
			os.Exit(2)
		}
		logrus.WithField("address", nodes).Info("Joining existing Serf nodes")
		serfConfig.JoinAddress = nodes
	}

	var serfNode *funk.SerfNode
	sleepTime := 2 * time.Second
	for {
		serfNode = funk.NewSerfNode()
		if err := serfNode.Start(config.NodeID, "", serfConfig); err != nil {
			logrus.WithError(err).Errorf("Unable to start Serf node. Will retry in %d seconds", sleepTime)
			time.Sleep(sleepTime)
			sleepTime *= 2
			if sleepTime > 32*time.Second {
				sleepTime = 32 * time.Second
			}
			continue
		}
		break
	}
	serfNode.SetTag(funk.SerfServiceName, "seed")
	serfNode.PublishTags()
	defer serfNode.Stop()

	if config.HTTP {
		addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
		if err != nil {
			logrus.WithError(err).Error("Could not resolve 0.0.0.0:0")
			os.Exit(2)
		}
		listener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			logrus.WithError(err).Error("Could not create listener")
			os.Exit(2)
		}
		httpAddr := fmt.Sprintf("http://%s/", netutils.ServiceHostPort(listener.Addr()))
		logrus.Infof("Launching web server on %s", httpAddr)
		go func() {
			time.Sleep(500 * time.Millisecond)
			open(httpAddr)
		}()
		http.HandleFunc("/", endpointPage(serfNode))

		if err := http.Serve(listener, nil); err != nil {
			logrus.WithError(err).Error("Got error launching HTTP server")
			os.Exit(2)
		}
	}
	if config.LiveView {
		for {
			clearScreen()
			dumpEndpoints(config.ClusterName, config.ShowAllNodes, serfNode.LoadMembers())
			spin()
		}
	}
	// Dump Serf events
	go func(evCh <-chan funk.NodeEvent) {
		for ev := range evCh {
			logrus.WithFields(logrus.Fields{
				"nodeId": ev.Node.NodeID,
				"event":  ev.Event.String(),
				"state":  ev.Node.State,
			}).Info("Serf event")
			for k, v := range ev.Node.Tags {
				logrus.WithField(k, v).Infof("Tags for node %s", ev.Node.NodeID)
			}
		}
	}(serfNode.Events())
	gotoolbox.WaitForSignal()

}
func dumpEndpoints(clusterName string, showAllNodes bool, members []funk.SerfMember) {
	sort.Slice(members, func(i, j int) bool {
		return members[i].NodeID < members[j].NodeID
	})

	fmt.Printf("Tags for cluster '%s'\n", clusterName)
	fmt.Printf("------------------------------------------------\n")

	for _, node := range members {
		if node.State != funk.SerfAlive && !showAllNodes {
			continue
		}
		fmt.Printf("Node: %s (%s)\n", node.NodeID, node.State)
		var tags []string
		for k := range node.Tags {
			tags = append(tags, k)
		}
		sort.Slice(tags, func(i, j int) bool {
			return tags[i] < tags[j]
		})
		for i, name := range tags {
			ch := '|'
			if i == (len(tags) - 1) {
				ch = '\\'
			}
			fmt.Printf("  %c- %s -> %s\n", ch, name, node.Tags[name])
		}
		fmt.Println()
	}

}

func clearScreen() {
	fmt.Print("\033c")
}

func spin() {
	fmt.Println()
	time.Sleep(1 * time.Second)
	fmt.Print("|\r")
	time.Sleep(1 * time.Second)
	fmt.Print("/\r")
	time.Sleep(1 * time.Second)
	fmt.Print("-\r")
	time.Sleep(1 * time.Second)
	fmt.Print("\\\r")
	time.Sleep(1 * time.Second)
}

type nodeList struct {
	Nodes   []nodeInfo `json:"nodes"`
	Updated string     `json:"updated"`
}

type nodeInfo struct {
	State string            `json:"state"`
	ID    string            `json:"nodeId"`
	Tags  map[string]string `json:"tags"`
}

func endpointPage(node *funk.SerfNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateSource := `
		<html>
		<head><title>Nodes in cluster</title></head>
		<style>
		body {
			font-size: 14pt;
			font-family: sans-serif;
		}
		h1 {
			font-size: 1.1em;
			background-color: lightblue;
			color: darkblue;
			padding-bottom: 0;
			margin-bottom: 0;
		}
		div.outer {
			columns: auto;
			column-width: 400px;
			column-rule: blue 1px solid;
		}
		div.node {
			width: 100%;
			display: inline-block;
			padding-bottom: 1em;
		}
		</style>
		<body>
		<div class="outer">
		{{range $i, $node := .Nodes }}
		<div class="node">
		<h1>{{$node.ID }} ({{$node.State}})</h1>
			<table>
			{{ range $name, $value := $node.Tags }}
			<tr><td>{{$name}}</td><td>{{$value}}</td></tr>
			{{ end }}
			</table>
		</div>
		{{end}}
		</div>
		<hr/>
		Refreshed at {{ .Updated }}
		</body>
		</html>
		`

		nodes := nodeList{
			Nodes:   make([]nodeInfo, 0),
			Updated: time.Now().String(),
		}

		members := node.LoadMembers()
		sort.Slice(members, func(i, j int) bool {
			return members[i].NodeID < members[j].NodeID
		})

		for _, n := range members {
			newNode := nodeInfo{
				State: n.State,
				ID:    n.NodeID,
				Tags:  make(map[string]string),
			}
			for k, v := range n.Tags {
				newNode.Tags[k] = v
			}
			nodes.Nodes = append(nodes.Nodes, newNode)
		}

		tmpl := template.Must(template.New("nodes").Parse(templateSource))
		tmpl.Execute(w, nodes)
	}
}

// open opens the specified URL in the default browser of the user.
func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

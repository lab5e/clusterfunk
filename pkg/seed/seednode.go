package seed

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	gotoolbox "github.com/lab5e/gotoolbox/toolbox"
	"github.com/sirupsen/logrus"
)

type parameters struct {
	ClusterName  string                  `kong:"help='Cluster name',default='clusterfunk'"`
	ZeroConf     bool                    `kong:"help='ZeroConf lookups for cluster',default='true'"`
	NodeID       string                  `kong:"help='Node ID for seed node',default=''"`
	Serf         funk.SerfParameters     `kong:"embed,prefix='serf-'"`
	Log          gotoolbox.LogParameters `kong:"embed,prefix='log-'"`
	LiveView     bool                    `kong:"help='Display live view of nodes',default='false'"`
	ShowAllNodes bool                    `kong:"help='Show all nodes, not just nodes alive',default='false'"`
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
	config.Serf.Final()

	logrus.WithField("nodeId", config.NodeID).Info("Starting seed node")

	if config.ZeroConf {
		zr := toolbox.NewZeroconfRegistry(config.ClusterName)
		_, port, err := net.SplitHostPort(config.Serf.Endpoint)

		if err != nil {
			logrus.WithError(err).WithField("hostport", config.Serf.Endpoint).Error("Host:port string is invalid")
			os.Exit(2)
		}
		portNum, err := strconv.Atoi(port)
		if err != nil {
			logrus.WithError(err).WithField("hostport", config.Serf.Endpoint).Error("Host:port string is invalid")
			os.Exit(2)
		}
		if err := zr.Register(funk.ZeroconfSerfKind, config.NodeID, portNum); err != nil {
			logrus.WithError(err).Error("Unable to register in ZeroConf")
			os.Exit(2)
		}
		defer zr.Shutdown()
	}

	serfNode := funk.NewSerfNode()
	if err := serfNode.Start(config.NodeID, config.Serf); err != nil {
		logrus.WithError(err).Error("Unable to start Serf node")
		os.Exit(2)
	}
	serfNode.PublishTags()
	defer serfNode.Stop()

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
			}).Debug("Serf event")
		}
	}(serfNode.Events())
	logrus.WithField("endpoint", config.Serf.Endpoint).Info("Seed node started")
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

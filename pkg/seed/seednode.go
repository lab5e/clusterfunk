package seed

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/lab5e/gotoolbox/netutils"
	gotoolbox "github.com/lab5e/gotoolbox/toolbox"
	"github.com/sirupsen/logrus"
)

type parameters struct {
	ClusterName     string                  `kong:"help='Cluster name',default='clusterfunk'"`
	ZeroConf        bool                    `kong:"help='ZeroConf lookups for cluster',default='true'"`
	NodeID          string                  `kong:"help='Node ID for seed node',default=''"`
	SerfPort        int                     `kong:"help='Port to use for Serf',default='0'"`
	SerfJoinAddress string                  `kong:"help='Join address for Serf', default=''"`
	Log             gotoolbox.LogParameters `kong:"embed,prefix='log-'"`
	LiveView        bool                    `kong:"help='Display live view of nodes',default='false'"`
	ShowAllNodes    bool                    `kong:"help='Show all nodes, not just nodes alive',default='false'"`
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
		Endpoint:    fmt.Sprintf("%s:%d", publicIP, config.SerfPort),
		Verbose:     (logrus.GetLevel() >= logrus.DebugLevel),
		JoinAddress: config.SerfJoinAddress,
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
	}

	var serfNode *funk.SerfNode
	sleepTime := 2 * time.Second
	for {
		serfNode := funk.NewSerfNode()
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

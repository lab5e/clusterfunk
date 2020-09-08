package ctrlc

import (
	"fmt"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk"
)

// ServicesCommand is a command to show cluster diagnostics
type ServicesCommand struct {
	ClusterName string              `kong:"help='Name of cluster',default='clusterfunk'"`
	ZeroConf    bool                `kong:"help='Use ZeroConf to discover Serf nodes',default='true'"`
	Serf        funk.SerfParameters `kong:"embed,prefix='serf-'"`
}

// Run executes the list service endpoints command
func (c *ServicesCommand) Run(args RunContext) error {
	params := funk.SerfParameters{}
	params.Final()

	param := args.ClusterCommands().Services
	client, err := clientfunk.NewClusterClient(
		param.ClusterName,
		param.ZeroConf,
		param.Serf.JoinAddress)
	if err != nil {
		return err
	}
	client.WaitForEndpoint(funk.RaftEndpoint)

	fmt.Printf("%-20s %s\n", "Name", "Listen address")
	fmt.Println("---------------------------------------------------------------")
	for _, v := range client.Endpoints() {
		fmt.Printf("%-20s %s\n", v.Name, v.ListenAddress)
	}
	return nil
}

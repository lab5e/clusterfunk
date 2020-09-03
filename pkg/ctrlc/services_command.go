package ctrlc

import (
	"fmt"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk"
)

// ServicesCommand is a command to show cluster diagnostics
type ServicesCommand struct {
	ZeroConf    bool                `kong:"help='Use ZeroConf to discover Serf nodes',default='true'"`
	ClusterName string              `kong:"help='Name of cluster',default='clusterfunk'"`
	Serf        funk.SerfParameters `kong:"embed,prefix='serf-'"`
}

// Run executes the list service endpoints command
func (c *ServicesCommand) Run(args RunContext) error {
	params := funk.SerfParameters{}
	params.Final()

	client, err := clientfunk.StartEndpointMonitor("ctrlc", args.ClusterCommands().Services.ClusterName, args.ClusterCommands().Services.ZeroConf, params)
	if err != nil {
		return err
	}
	client.WaitForEndpoints()
	fmt.Printf("%-20s %s\n", "Name", "Listen address")
	fmt.Println("---------------------------------------------------------------")
	for _, v := range client.ServiceEndpoints() {
		fmt.Printf("%-20s %s\n", v.Name, v.ListenAddress)
	}
	return nil
}

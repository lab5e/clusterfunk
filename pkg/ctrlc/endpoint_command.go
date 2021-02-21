package ctrlc

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk"
)

// EndpointsCommand is the subcommand to list endpoints
type EndpointsCommand struct {
	Filter       string `kong:"help='Filter on endpoint or service name',default=''"`
	Service      string `kong:"help='Filter on service name',default=''"`
	ShowInactive bool   `kong:"help='Include inactive nodes',default='false'"`
	Format       string `kong:"help='Output format',enum='tree,plain,json,namevalue',default='tree'"`
}

type endpointInfo struct {
	Name          string `json:"name"`
	ListenAddress string `json:"listenAddress"`
}

type endpointList struct {
	NodeID    string         `json:"nodeId"`
	Service   string         `json:"service"`
	Endpoints []endpointInfo `json:"endpoints"`
}

func (c *EndpointsCommand) retrieveEndpoints(params clientfunk.ClientParameters) (map[string]endpointList, error) {
	client, err := clientfunk.NewClusterClient(params)
	if err != nil {
		return nil, err
	}

	// Group everything into a list for each node
	services := make(map[string]endpointList)

	for _, v := range client.Endpoints() {
		if c.filter(v) {
			continue
		}
		node, ok := services[v.NodeID]
		if !ok {
			node = endpointList{
				NodeID:    v.NodeID,
				Service:   v.Service,
				Endpoints: make([]endpointInfo, 0),
			}
		}
		ep := endpointInfo{Name: v.Name, ListenAddress: v.ListenAddress}
		node.Endpoints = append(node.Endpoints, ep)
		services[v.NodeID] = node
	}
	return services, nil
}

func (c *EndpointsCommand) filter(ep funk.Endpoint) bool {
	// Skip inactive nodes unless requested
	if !ep.Active && !c.ShowInactive {
		return true
	}
	// Apply filter if it's set
	if c.Filter != "" && !strings.Contains(ep.Name, c.Filter) {
		return true

	}
	if c.Service != "" && !strings.Contains(ep.Service, c.Service) {
		return true
	}
	return false
}

func (c *EndpointsCommand) printList(list map[string]endpointList, format string) {
	// Convert the map into an array and sort on node name
	nodes := make([]endpointList, 0)
	for _, v := range list {
		nodes = append(nodes, v)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})

	switch format {
	case "json":
		json.NewEncoder(os.Stdout).Encode(list)
	case "plain":
		table := tabwriter.NewWriter(os.Stdout, 3, 4, 3, ' ', 0)
		table.Write([]byte("Node\tService\tName\tEndpoint\n"))
		for _, node := range nodes {
			epList := node.Endpoints
			sort.Slice(epList, func(i, j int) bool {
				return epList[i].Name < epList[j].Name
			})
			for _, ep := range epList {
				table.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\n", node.NodeID, node.Service, ep.Name, ep.ListenAddress)))
			}
		}
		table.Flush()
	case "tree":
		fmt.Printf("------------------------------------------------\n")
		for _, node := range nodes {
			fmt.Printf("Node: %s, Service: %s\n", node.NodeID, node.Service)
			epList := node.Endpoints
			sort.Slice(epList, func(i, j int) bool {
				return epList[i].Name < epList[j].Name
			})
			for i, ep := range epList {
				ch := '|'
				if i == (len(epList) - 1) {
					ch = '\\'
				}
				fmt.Printf("  %c- %s -> %s\n", ch, ep.Name, ep.ListenAddress)
			}
			fmt.Println()
		}
	case "namevalue":
		for _, node := range nodes {
			for _, ep := range node.Endpoints {
				fmt.Printf("%s.%s.%s=%s\n", node.NodeID, node.Service, ep.Name, ep.ListenAddress)
			}
		}
	default:
		panic("Unknown output format: " + format)
	}

}

// Run executes the endpoint command
func (c *EndpointsCommand) Run(args RunContext) error {
	list, err := c.retrieveEndpoints(args.ClientParams())
	if err != nil {
		fmt.Printf("Error connecting to cluster: %v\n", err)
		return err
	}
	c.printList(list, c.Format)
	return nil
}

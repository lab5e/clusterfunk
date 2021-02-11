package ctrlc

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/grandcat/zeroconf"
)

// DiagnosticsCommand is a command to show cluster diagnostics
type DiagnosticsCommand struct {
}

// Run executes the diagnostic command
func (c *DiagnosticsCommand) Run(args RunContext) error {
	if args.ClusterServer().Zeroconf {
		// Dump zeroconf lookups. Listen for 5 seconds
		fmt.Println("Zeroconf lookup (5 seconds)...:")
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			fmt.Printf("Error creating zeroconf resolver: %v\n", err)
			return err
		}
		entries := make(chan *zeroconf.ServiceEntry)
		waitTime := time.Second * 5
		go func(entries chan *zeroconf.ServiceEntry) {
			ctx, cancel := context.WithTimeout(context.Background(), waitTime)
			defer cancel()
			err = resolver.Browse(ctx, "_clusterfunk._udp", "local.", entries)
			if err != nil {
				close(entries)
				return
			}
			<-ctx.Done()
		}(entries)

		table := tabwriter.NewWriter(os.Stdout, 1, 3, 1, ' ', 0)
		table.Write([]byte("Instance\tHostName\tPort\tIPv4\tIPv6\n"))
		for entry := range entries {
			ipv4 := ""
			ipv6 := ""
			if len(entry.AddrIPv4) > 0 {
				ipv4 = entry.AddrIPv4[0].String()
			}
			if len(entry.AddrIPv6) > 0 {
				ipv6 = entry.AddrIPv6[0].String()
			}
			table.Write([]byte(fmt.Sprintf("%s\t%s\t%d\t%s\t%s\n", entry.ServiceInstanceName(), entry.HostName, entry.Port, ipv4, ipv6)))
		}
		table.Flush()
	}

	return nil
}

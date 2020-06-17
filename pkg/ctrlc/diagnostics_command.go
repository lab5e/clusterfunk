package ctrlc

import (
	"context"
	"fmt"
	"time"

	"github.com/grandcat/zeroconf"
)

type DiagnosticsCommand struct {
}

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

		fmt.Printf("%-60s %-20s %-6s %-16s %s\n", "Instance", "HostName", "Port", "IPv4", "IPv6")
		for entry := range entries {
			ipv4 := ""
			ipv6 := ""
			if len(entry.AddrIPv4) > 0 {
				ipv4 = entry.AddrIPv4[0].String()
			}
			if len(entry.AddrIPv6) > 0 {
				ipv6 = entry.AddrIPv6[0].String()
			}
			fmt.Printf("%-60s %-20s %-6d %-16s %s\n", entry.ServiceInstanceName(), entry.HostName, entry.Port, ipv4, ipv6)
		}
	}

	return nil
}

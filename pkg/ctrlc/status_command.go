package ctrlc

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

// StatusCommand is a subcommand for the ctrlc CLI.
type StatusCommand struct {
}

// Run executes the status operation
func (c *StatusCommand) Run(args RunContext) error {
	client := connectToManagement(args.ClientParams())
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()
	res, err := client.GetStatus(ctx, &managepb.GetStatusRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving status: %v\n", err)
		return errStd
	}
	t := time.Unix(0, res.Created)
	d := time.Since(t)
	table := tabwriter.NewWriter(os.Stdout, 4, 4, 3, ' ', 0)
	table.Write([]byte(fmt.Sprintf("Cluster name:\t%s\n", res.ClusterName)))
	table.Write([]byte(fmt.Sprintf("Created at\t%s (%dd, %02dh, %02dm, %02ds ago)\n", t.Format(time.RFC822),
		int64(d.Hours()/24), int64(d.Hours()), int64(d.Minutes()), int64(d.Seconds()))))
	table.Write([]byte(fmt.Sprintf("Node ID:\t%s\n", res.LocalNodeId)))
	table.Write([]byte(fmt.Sprintf("State:\t%s\n", res.LocalState)))
	table.Write([]byte(fmt.Sprintf("Role:\t%s\n", res.LocalRole)))
	table.Write([]byte(fmt.Sprintf("Leader ID:\t%s\n", res.LeaderNodeId)))
	table.Write([]byte(fmt.Sprintf("Nodes:\t%d Raft, %d Serf\n", res.RaftNodeCount, res.SerfNodeCount)))
	table.Write([]byte(fmt.Sprintf("Shards:\t%d\n", res.ShardCount)))
	table.Write([]byte(fmt.Sprintf("Your IP:\t%s\n", res.YourIp)))
	table.Flush()
	return nil
}

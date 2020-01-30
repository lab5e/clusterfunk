package ctrlc

import (
	"context"
	"fmt"
	"os"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/clustermgmt"
)

// StepDownCommand is the step-down subcommand. The current leader will step
// down and let another node assume leadership.
type StepDownCommand struct {
}

// Run executes the step-down command on the node
func (c *StepDownCommand) Run(param *Command) error {
	client := connectToManagement(param)
	if client == nil {
		return errStd
	}

	ctx, done := context.WithTimeout(context.Background(), gRPCTimeout)
	defer done()

	res, err := client.StepDown(ctx, &clustermgmt.StepDownRequest{})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error asking leader to step down: %v\n", err)
		return errStd
	}
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "Leader is unable to step down: %v\n", res.Error.Message)
		return errStd
	}
	fmt.Printf("Leader node %s has stepped down\n", res.NodeId)

	return nil
}

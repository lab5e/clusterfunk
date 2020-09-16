package ctrlc

import (
	"fmt"
	"os"
	"time"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

const gRPCTimeout = 10 * time.Second

func connectToManagement(params clientfunk.ManagementServerParameters) managepb.ClusterManagementClient {
	client, err := clientfunk.ConnectToManagement(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return nil
	}
	return client
}

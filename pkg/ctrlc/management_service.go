package ctrlc

import (
	"fmt"
	"os"
	"time"

	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
)

const gRPCTimeout = 10 * time.Second

func connectToManagement(params clientfunk.ClientParameters) managepb.ClusterManagementClient {
	client, err := clientfunk.NewClusterClient(params)
	if err != nil {
		fmt.Printf("Could not create cluster client: %v", err)
		return nil
	}
	mgmtClient, err := clientfunk.ConnectToManagement(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return nil
	}
	return mgmtClient
}

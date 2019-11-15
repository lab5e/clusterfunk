package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/ExploratoryEngineering/params"

	"github.com/stalehd/clusterfunk/pkg/toolbox"

	"github.com/aclements/go-moremath/stats"
	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/pkg/clientfunk"
)

const grpcServiceEndpointName = "ep.demo"

type parameters struct {
	Endpoints    string        `param:"desc=Comma-separated list of endpoints to use. Will use zeroconf to find the parameters"`
	ClusterName  string        `param:"desc=Cluster name;default=clusterfunk"`
	Repeats      int           `param:"desc=Number of times to repeat rpc call;default=50"`
	Sleep        time.Duration `param:"desc=Sleep between invocations;default=100ms"`
	PrintSummary bool          `param:"desc=Print summary when finished;default=true"`
}

var config parameters

func singleCall(conn *grpc.ClientConn) *demo.LiffResponse {
	liffClient := demo.NewDemoServiceClient(conn)
	ctx, done := context.WithTimeout(context.Background(), 1*time.Second)
	defer done()
	res, err := liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
	if err != nil {
		fmt.Printf("Error calling Liff: %v\n", err)
		return nil
	}
	return res
}

func main() {
	if err := params.NewEnvFlag(&config, os.Args[1:]); err != nil {
		fmt.Println(err)
		return
	}

	// Seed is quite important here
	rand.Seed(time.Now().UnixNano())

	if !updateEndpoints(config.ClusterName, config.Endpoints) {
		fmt.Println("Could not get endpoints")
		return
	}

	stats := make(map[string]stats.StreamStats)

	allStart := time.Now()
	grpcConnection, err := grpc.Dial("cluster:///ep.demo", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))
	if err != nil {
		fmt.Println("Unable to get connection: ", err)
		return
	}
	defer grpcConnection.Close()

	for i := 0; i < config.Repeats; i++ {
		waitCh := time.After(config.Sleep)
		if i%10 == 0 {
			fmt.Print(".")
		}

		start := time.Now()
		res := singleCall(grpcConnection)
		duration := time.Since(start)
		if res != nil {
			s := stats[res.NodeID]
			s.Add(float64(duration) / float64(time.Millisecond))
			stats[res.NodeID] = s
		}
		<-waitCh
	}
	fmt.Println()
	totalDuration := time.Since(allStart)
	if config.PrintSummary {
		fmt.Println("\n=================================================")
		total := uint(0)
		for k, v := range stats {
			fmt.Printf("%20s: %d items min: %6.3f  max: %6.3f  mean: %6.3f  stddev: %6.3f\n", k, v.Count, v.Min, v.Max, v.Mean(), v.StdDev())
			total += v.Count
		}
		fmt.Printf("%d in total, %d with errors\n", config.Repeats, config.Repeats-int(total))
		fmt.Printf("%6.3f reqs/sec on average\n", float64(config.Repeats)/(float64(totalDuration)/float64(time.Second)))
	}
}

func updateEndpoints(clusterName, configuredEndpoints string) bool {
	ep, err := clientfunk.ZeroconfManagementLookup(clusterName)
	if err != nil {
		fmt.Printf("Unable to do zeroconf lookup for cluster %s: %v\n", clusterName, err)
		return false
	}

	var eps []string

	if configuredEndpoints != "" {
		eps = strings.Split(configuredEndpoints, ",")
		clientfunk.UpdateClusterEndpoints(grpcServiceEndpointName, eps)
	} else {
		eps, err = clientfunk.GetEndpoints(grpcServiceEndpointName, toolbox.GRPCClientParam{ServerEndpoint: ep})
		if err != nil {
			fmt.Printf("Unable to locate endpoints: %v\n", err)
			return false
		}
		clientfunk.UpdateClusterEndpoints(grpcServiceEndpointName, eps)
	}
	return true
}

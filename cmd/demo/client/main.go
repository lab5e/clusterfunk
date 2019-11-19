package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/ExploratoryEngineering/params"

	"github.com/aclements/go-moremath/stats"
	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/pkg/clientfunk"
	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

type parameters struct {
	Endpoints    string        `param:"desc=Comma-separated list of endpoints to use. Will use zeroconf to find the parameters"`
	ClusterName  string        `param:"desc=Cluster name;default=clusterfunk"`
	Repeats      int           `param:"desc=Number of times to repeat rpc call;default=50"`
	Sleep        time.Duration `param:"desc=Sleep between invocations;default=100ms"`
	PrintSummary bool          `param:"desc=Print summary when finished;default=true"`
	ZeroConf     bool          `param:"desc=ZeroConf lookups for cluster;default=true"`
	Serf         funk.SerfParameters
}

var config parameters

func main() {
	// Get the configuration. The defaults will run 50 calls, one call every 100 ms.
	if err := params.NewEnvFlag(&config, os.Args[1:]); err != nil {
		fmt.Println(err)
		return
	}

	// Set up the progress bar and stat counters
	progress := toolbox.ConsoleProgress{Max: config.Repeats}
	stats := make(map[string]stats.StreamStats)

	// The endpoint monitor keeps the list of endpoints up to date by monitoring
	// the Serf nodes in the cluster. When a new endpoint appears it will be picked
	// up by the resolver and used by the client.
	em, err := clientfunk.StartEndpointMonitor("", config.ClusterName, config.ZeroConf, config.Serf)
	if err != nil {
		fmt.Printf("Unable to start endpoint monitor: %v\n", err)
		return
	}
	defer em.Stop()

	// Wait for the list of endpoints to be ready. It might take a few seconds for
	// Serf to retrieve the list of nodes.
	em.WaitForEndpoints()

	// Start timing the whole process. It won't be very accurat but it should give
	// a nice approximatino.
	startRun := time.Now()

	// Create the connection. The target is quite simple and the resolver inside
	// the clientfunk package will resolve this to the actual IP adresses and
	// ports of the cluster. The service configuration ensures us that we're
	// using a round robin load balancer.
	grpcConnection, err := grpc.Dial("cluster:///ep.demo",
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))

	if err != nil {
		fmt.Println("Unable to get connection: ", err)
		return
	}
	defer grpcConnection.Close()

	// Start the calls.
	success := 0
	for i := 0; i < config.Repeats; i++ {
		waitCh := time.After(config.Sleep)

		progress.Print(i + 1)

		// This is used to measure the time for the call. The results will be
		// shown at the end.
		start := time.Now()

		// The call is quite straightforward. The timeout is set to 1 second
		// so it should handle the cluster resharding without problems. There
		// *will* be issues when a node goes away since the client calls happens
		// at a steady rate but only the clients currently connected to the node
		// that fails are affected.
		liffClient := demo.NewDemoServiceClient(grpcConnection)
		ctx, done := context.WithTimeout(context.Background(), 1*time.Second)
		res, err := liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
		done()

		if err != nil {
			fmt.Println()
			fmt.Println(err.Error())
			fmt.Println()
		} else {
			success++
		}

		// Record stats for call.
		duration := time.Since(start)
		s := stats[res.NodeID]
		s.Add(float64(duration) / float64(time.Millisecond))
		stats[res.NodeID] = s

		<-waitCh
	}

	totalDuration := time.Since(startRun)

	// ...and print a summary
	if config.PrintSummary {
		total := uint(0)
		for k, v := range stats {
			fmt.Printf("%20s: %d items min: %6.3f  max: %6.3f  mean: %6.3f  stddev: %6.3f\n", k, v.Count, v.Min, v.Max, v.Mean(), v.StdDev())
			total += v.Count
		}
		fmt.Printf("%d in total, %d with errors\n", config.Repeats, config.Repeats-int(total))
		fmt.Printf("%6.3f reqs/sec on average\n", float64(config.Repeats)/(float64(totalDuration)/float64(time.Second)))
	}
}

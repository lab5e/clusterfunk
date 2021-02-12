package main

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/aclements/go-moremath/stats"
	"github.com/alecthomas/kong"
	"github.com/lab5e/clusterfunk/cmd/demo"
	"github.com/lab5e/clusterfunk/pkg/clientfunk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
)

type parameters struct {
	Endpoints    string                      `kong:"help='Comma-separated list of endpoints to use. Will use zeroconf to find the parameters'"`
	Repeats      int                         `kong:"help='Number of times to repeat rpc call',default='50'"`
	Sleep        time.Duration               `kong:"help='Sleep between invocations',default='100ms'"`
	PrintSummary bool                        `kong:"help='Print summary when finished',default='true'"`
	Cluster      clientfunk.ClientParameters `kong:"embed,prefix='cluster-',help='Cluster parameters'"`
	Retry        bool                        `kong:"help='Do a single retry for failed requests',default='true'"`
}

var config parameters

func main() {
	k, err := kong.New(&config, kong.Name("client"),
		kong.Description("Demo client"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: false,
		}))
	if err != nil {
		panic(err)
	}
	if _, err := k.Parse(os.Args[1:]); err != nil {
		k.FatalIfErrorf(err)
		return
	}
	// Set up the progress bar and stat counters
	progress := toolbox.ConsoleProgress{Max: config.Repeats}
	stats := make(map[string]stats.StreamStats)

	// The endpoint monitor keeps the list of endpoints up to date by monitoring
	// the Serf nodes in the cluster. When a new endpoint appears it will be picked
	// up by the resolver and used by the client.
	client, err := clientfunk.NewClusterClient(config.Cluster)
	if err != nil {
		fmt.Printf("Unable to start cluster client monitor: %v\n", err)
		return
	}

	// Wait for the list of endpoints to be ready. It might take a few seconds for
	// Serf to retrieve the list of nodes.
	client.WaitForEndpoint("ep.demo")

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

	// Since we're going to do a single retry we'll just make a function that
	// we call multiple times.
	liffCall := func() (*demo.LiffResponse, error) {
		// The call is quite straightforward. The timeout is set to 1 second
		// so it should handle the cluster resharding without problems. There
		// *will* be issues when a node goes away since the client calls happens
		// at a steady rate but only the clients currently connected to the node
		// that fails are affected.
		liffClient := demo.NewDemoServiceClient(grpcConnection)
		ctx, done := context.WithTimeout(context.Background(), 1*time.Second)
		defer done()
		return liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
	}

	// Start the calls.
	success := 0
	retries := 0
	for i := 0; i < config.Repeats; i++ {
		waitCh := time.After(config.Sleep)

		progress.Print(i + 1)

		// This is used to measure the time for the call. The results will be
		// shown at the end.
		start := time.Now()
		res, err := liffCall()
		if err != nil {
			retries++
			res, err = liffCall()
		}
		duration := time.Since(start)

		if err != nil {
			fmt.Println()
			fmt.Println(err.Error())
			fmt.Println()
		} else {
			s := stats[res.NodeID]
			s.Add(float64(duration) / float64(time.Millisecond))
			stats[res.NodeID] = s
			success++
		}

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
		fmt.Printf("%d in total, %d retries, %d with errors\n", config.Repeats, retries, config.Repeats-success)
		fmt.Printf("%6.3f reqs/sec on average\n", float64(config.Repeats)/(float64(totalDuration)/float64(time.Second)))
	}
}

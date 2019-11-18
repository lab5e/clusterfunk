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
	if err := params.NewEnvFlag(&config, os.Args[1:]); err != nil {
		fmt.Println(err)
		return
	}

	stats := make(map[string]stats.StreamStats)

	// The endpoint monitor keeps the list of endpoints up to date by monitoring the Serf nodes in the cluster.
	em, err := clientfunk.StartEndpointMonitor("", config.ClusterName, config.ZeroConf, config.Serf)
	if err != nil {
		fmt.Printf("Unable to start endpoint monitor: %v\n", err)
		return
	}
	defer em.Stop()
	fmt.Println("Waiting for endpoints")
	em.WaitForEndpoints()
	fmt.Println("Endpoints added")
	allStart := time.Now()
	grpcConnection, err := grpc.Dial("cluster:///ep.demo",
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))

	if err != nil {
		fmt.Println("Unable to get connection: ", err)
		return
	}
	defer grpcConnection.Close()

	success := 0
	for i := 0; i < config.Repeats; i++ {
		waitCh := time.After(config.Sleep)
		if i%10 == 0 {
			fmt.Print(".")
		}

		start := time.Now()
		liffClient := demo.NewDemoServiceClient(grpcConnection)
		ctx, done := context.WithTimeout(context.Background(), 1*time.Second)
		res, err := liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
		done()
		duration := time.Since(start)
		if err != nil {
			fmt.Println("Error: ", err)
		} else {
			s := stats[res.NodeID]
			s.Add(float64(duration) / float64(time.Millisecond))
			stats[res.NodeID] = s
			success++
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

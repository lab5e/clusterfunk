package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/ExploratoryEngineering/params"

	"github.com/stalehd/clusterfunk/pkg/toolbox"

	"github.com/aclements/go-moremath/stats"
	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/pkg/clientfunk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const grpcServiceEndpointName = "ep.demo"

type parameters struct {
	Endpoints    string        `param:"desc=Comma-separated list of endpoints to use. Will use zeroconf to find the parameters"`
	ClusterName  string        `param:"desc=Cluster name;default=clusterfunk"`
	Repeats      int           `param:"desc=Number of times to repeat rpc call;default=50"`
	Sleep        time.Duration `param:"desc=Sleep between invocations;default=100ms"`
	LogTiming    bool          `param:"desc=Log timings to a CSV file;default=false"`
	PrintSummary bool          `param:"desc=Print summary when finished;default=true"`
	NumWorkers   int           `param:"desc=Number of workers to run;default=3"`
}

var config parameters

type result struct {
	Time    float64
	NodeID  string
	Success bool
}

func main() {
	if err := params.NewEnvFlag(&config, os.Args[1:]); err != nil {
		fmt.Println(err)
		return
	}

	// Seed is quite important here
	rand.Seed(time.Now().UnixNano())

	wg := &sync.WaitGroup{}
	wg.Add(config.NumWorkers)
	// Timings are reported as positive values for successful calls, negative
	// otherwise.
	timings := make(chan result)

	ep, err := clientfunk.ZeroconfManagementLookup(config.ClusterName)
	if err != nil {
		panic(fmt.Sprintf("Unable to do zeroconf lookup for cluster %s: %v", config.ClusterName, err))
	}

	var eps []string

	if config.Endpoints != "" {
		eps = strings.Split(config.Endpoints, ",")
		clientfunk.UpdateClusterEndpoints(grpcServiceEndpointName, eps)
	} else {
		eps, err = clientfunk.GetEndpoints(grpcServiceEndpointName, toolbox.GRPCClientParam{ServerEndpoint: ep})
		if err != nil {
			panic(fmt.Sprintf("Unable to locate endpoints: %v", err))
		}
		clientfunk.UpdateClusterEndpoints(grpcServiceEndpointName, eps)
	}

	// Notice the three slashes here. It's *really* important. If you use a custom scheme in gRPC the authority field must be included
	grpcConnection, err := grpc.Dial("cluster:///ep.demo", grpc.WithInsecure(), grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))
	if err != nil {
		panic(err)
	}
	liffClient := demo.NewDemoServiceClient(grpcConnection)
	if err != nil {
		panic(fmt.Sprintf("Unable to create"))
	}
	for worker := 0; worker < config.NumWorkers; worker++ {
		go func(timingCh chan<- result, times int) {
			for i := 0; i < times; i++ {
				time.Sleep(config.Sleep)
				ctx, done := context.WithTimeout(context.Background(), 3*time.Second)
				start := time.Now()
				res, err := liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
				stop := time.Now()
				done()
				if err != nil {
					code := status.Code(err)
					switch code {
					case codes.Unavailable /*, codes.DeadlineExceeded*/ :
						fmt.Printf("Connection %s might be unhealthy (deadline)\n", grpcConnection.Target())
					default:
						fmt.Fprintf(os.Stderr, "Error calling Liff: %v (code=%v)\n", err, code)
					}
					timingCh <- result{Success: false}
					continue
				}

				timingCh <- result{
					NodeID:  res.NodeID,
					Success: true,
					Time:    float64(stop.Sub(start)) / float64(time.Millisecond)}

			}
			wg.Done()
		}(timings, config.Repeats)
	}

	var csvFile *os.File
	if config.LogTiming {
		var err error
		csvFile, err = os.Create("timing.csv")
		if err != nil {
			fmt.Printf("Can't create file timing.csv: %v\n", err)
			return
		}
		fmt.Fprintf(csvFile, "Num,Node,Time\n")
		defer csvFile.Close()
	}

	stats := make(map[string]stats.StreamStats)
	errors := 0
	itemNo := 0
	go func() {
		for result := range timings {
			itemNo++
			fmt.Printf("\rReceived %d responses", itemNo)
			if !result.Success {
				errors++
				fmt.Printf("Error     : %f ms\n", result.Time)
				continue
			}
			s := stats[result.NodeID]
			s.Add(result.Time)
			stats[result.NodeID] = s
			// Log call time to CSV
			fmt.Fprintf(csvFile, "%d,%s,%f\n", itemNo, result.NodeID, result.Time)
		}
	}()

	wg.Wait()
	// Wait for the reader to finish. Not pretty but I'll fix it.
	time.Sleep(100 * time.Millisecond)

	if config.PrintSummary {

		fmt.Println("\n=================================================")
		total := uint(0)
		for k, v := range stats {
			fmt.Printf("%20s: %d items min: %6.3f  max: %6.3f  mean: %6.3f  stddev: %6.3f\n", k, v.Count, v.Min, v.Max, v.Mean(), v.StdDev())
			total += v.Count
		}
		fmt.Printf("%d in total, %d with errors\n", total, errors)
	}
}

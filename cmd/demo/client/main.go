package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/stalehd/clusterfunk/toolbox"

	"github.com/stalehd/clusterfunk/clientfunk"
	"github.com/stalehd/clusterfunk/cmd/demo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type parameters struct {
	Endpoints    string
	Repeats      int
	Sleep        time.Duration
	LogTiming    bool
	PrintSummary bool
}

var config parameters

func init() {
	flag.StringVar(&config.Endpoints, "endpoints", "", "Comma-separated list of host:port pairs to use")
	flag.IntVar(&config.Repeats, "repeat", 10, "Number of times to repeat the command")
	flag.DurationVar(&config.Sleep, "sleep", 100*time.Millisecond, "Time to sleep between calls")
	flag.BoolVar(&config.LogTiming, "log", true, "Log timings to a CSV file")
	flag.BoolVar(&config.PrintSummary, "print-summary", true, "Print summary when finished")
	flag.Parse()

}
func main() {

	endpoints := strings.Split(config.Endpoints, ",")
	if config.Endpoints == "" {
		ep, err := clientfunk.ZeroconfManagementLookup("demo")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to do zeroconf lookup: %v\n", err)
			return
		}
		endpoints, err = clientfunk.GetEndpoints("ep.demo", toolbox.GRPCClientParam{ServerEndpoint: ep})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to locate endpoints: %v\n", err)
		}
	}
	clients := make([]demo.DemoServiceClient, 0)

	for _, v := range endpoints {
		opts := []grpc.DialOption{grpc.WithInsecure()}
		conn, err := grpc.Dial(v, opts...)
		if err != nil {
			fmt.Printf("Error dialing to client: %v\n", err)
			return
		}
		defer conn.Close()
		newClient := demo.NewDemoServiceClient(conn)
		if newClient == nil {
			panic("Client is nil")
		}
		clients = append(clients, newClient)
	}
	if len(clients) == 0 {
		fmt.Printf("No endpoints specified")
		return
	}

	nodes := make(map[string]int)
	totals := make(map[string]float64)

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

	success := 0
	clientNum := 0
	// Seed is quite important here
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < config.Repeats; i++ {
		if i > 0 {
			time.Sleep(config.Sleep)
		}
		requestid := int64(rand.Int())

		ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
		start := time.Now()
		var end time.Time

		client := clients[clientNum%len(clients)]
		clientNum++
		resp, err := client.Liff(ctx, &demo.LiffRequest{ID: requestid})
		end = time.Now()
		done()
		if err != nil {
			fmt.Printf("Error calling Liff: %v\n", err)
			if config.Repeats == 1 {
				return
			}
			continue
		}
		end.Sub(start)
		callTime := float64(end.Sub(start)) / float64(time.Millisecond)
		if resp != nil {
			success++
			fmt.Printf("Time on %s: %f ms\n", resp.NodeID, callTime)

			nodes[resp.NodeID]++
			totals[resp.NodeID] += callTime
			// Log call time to CSV
			fmt.Fprintf(csvFile, "%d,%s,%f\n", i, resp.NodeID, callTime)
		}
	}
	if config.PrintSummary {
		fmt.Println("=================================================")
		fmt.Printf("%d calls in total, %d successful, %d failed\n", config.Repeats, success, config.Repeats-success)
		for k, v := range nodes {
			fmt.Printf("%20s: %d calls - %f ms average\n", k, v, totals[k]/float64(v))
		}
	}
}

// No need to test the conversion since we're providing both lists but if you
// want to make *really* sure use the
func demoServerCall(ctx context.Context, client interface{}, parameter interface{}) (interface{}, error) {
	// This isn't necessary but it's nice for debugging
	c, ok := client.(demo.DemoServiceClient)
	if !ok {
		panic("supplied client isn't a DemoServerClient")
	}
	p, ok := parameter.(*demo.LiffRequest)
	if !ok {
		panic("supplied parameter isn't LiffRequest")
	}
	return c.Liff(ctx, p)

	// The above could be a one liner like this:
	// return client.(demo.DemoServiceClient).Liff(ctx, parameter.(*demo.LiffRequest))
}

// The below will be moved to the clientfunk package later
type serverCall func(ctx context.Context, client interface{}, parameter interface{}) (interface{}, error)

// Do repeated calls to different clients. If the first doesn't respond within
// 50 ms try the next with 100 ms timeout, then the final with 200 ms timeout.
// if all three fails return with an error.

func failoverCall(ctx context.Context, clients []interface{}, call serverCall, param interface{}) (interface{}, error) {
	retCh := make(chan interface{})
	errCh := make(chan error)

	// This is the calling goroutine. It returns the error and response on the channels
	doCall := func(ctx context.Context, client interface{}, call serverCall, respCh chan<- interface{}, errCh chan<- error) {
		resp, err := call(ctx, client, param)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}

	// Start with a 32 ms timeout and double for each client
	timeout := time.Millisecond * 32
	// Keep on trying until either successful, there's an error or
	// we run out of clients.
	for len(clients) > 0 {
		// Use the double timeout or we'd get an error right away if the first
		// client doesn't respond
		ctx, cancel := context.WithTimeout(ctx, timeout*2)
		defer cancel()
		go doCall(ctx, clients[0], call, retCh, errCh)
		select {
		case resp := <-retCh:
			// Great success. The client returned a
			return resp, nil
		case err := <-errCh:
			code := status.Code(err)
			if code == codes.Unavailable {
				// Service is down, try another client
				break
			}
			// Any other gRPC error is just passed along.
			return nil, err
		case <-time.After(timeout):
			// Next client
		}
		clients = clients[1:]
		timeout *= 2
	}

	return nil, errors.New("All clients timed out")
}

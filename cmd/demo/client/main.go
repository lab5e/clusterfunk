package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

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
	PrintResult  bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s [endpoint] [repeats, default 10]\n", os.Args[0])
		return
	}
	ep := os.Args[1]

	repeats := 10
	if len(os.Args) > 2 {
		v, err := strconv.ParseInt(os.Args[2], 10, 32)
		if err != nil {
			fmt.Println("Invalid number of repeats")
			return
		}
		repeats = int(v)
	}
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(ep, opts...)
	if err != nil {
		fmt.Printf("Error dialing to client: %v\n", err)
		return
	}
	defer conn.Close()

	client := demo.NewDemoServiceClient(conn)

	nodes := make(map[string]int)
	totals := make(map[string]float64)
	sleepTime := time.Second * 1
	if repeats >= 100 {
		sleepTime = 100 * time.Millisecond
	}
	csvFile, err := os.Create("timing.csv")
	if err != nil {
		fmt.Printf("Can't create file timing.csv: %v\n", err)
		return
	}
	fmt.Fprintf(csvFile, "Num,Node,Time\n")

	for i := 0; i < repeats; i++ {
		if i > 0 {
			time.Sleep(sleepTime)
		}
		// Seed is quite important here
		rand.Seed(time.Now().UnixNano())
		requestid := int64(rand.Int())

		ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		start := time.Now()
		var end time.Time
		var nodeid string

		resp, err := client.Liff(ctx, &demo.LiffRequest{ID: requestid})
		end = time.Now()
		if err != nil {
			fmt.Printf("Error calling Liff: %v\n", err)
			if repeats == 1 {
				return
			}
		} else {
			nodeid = resp.NodeID
			if repeats == 1 {
				fmt.Println(resp.Definition)
			}
		}

		end.Sub(start)
		callTime := float64(end.Sub(start)) / float64(time.Millisecond)
		if nodeid != "" {
			fmt.Printf("Time on %s: %f ms\n", nodeid, callTime)

			nodes[nodeid]++
			totals[nodeid] += callTime
			// Log call time to CSV
			fmt.Fprintf(csvFile, "%d,%s,%f\n", i, nodeid, callTime)
		}
	}
	fmt.Println("=================================================")
	for k, v := range nodes {
		fmt.Printf("%20s: %d calls - %f ms average\n", k, v, totals[k]/float64(v))
	}
	csvFile.Close()
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

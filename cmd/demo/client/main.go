package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
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

const numWorkers = 1

var config parameters

func init() {
	flag.IntVar(&config.Repeats, "repeat", 50, "Number of times to repeat the command")
	flag.DurationVar(&config.Sleep, "sleep", 100*time.Millisecond, "Time to sleep between calls")
	flag.BoolVar(&config.LogTiming, "log", true, "Log timings to a CSV file")
	flag.BoolVar(&config.PrintSummary, "print-summary", true, "Print summary when finished")
	flag.Parse()
}

type result struct {
	Time    float64
	NodeID  string
	Success bool
}

func refreshPool(pool *clientfunk.ClientPool) {
	ep, err := clientfunk.ZeroconfManagementLookup("demo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to do zeroconf lookup: %v\n", err)
		return
	}
	endpoints, err := clientfunk.GetEndpoints("ep.demo", toolbox.GRPCClientParam{ServerEndpoint: ep})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to locate endpoints: %v\n", err)
	}
	var p []string
	for i := 0; i < 3; i++ {
		p = append(p, endpoints...)
	}
	pool.Sync(p)
}
func main() {
	clientPool := clientfunk.NewClientPool([]grpc.DialOption{grpc.WithInsecure()})
	refreshPool(clientPool)

	// Seed is quite important here
	rand.Seed(time.Now().UnixNano())

	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// Timings are reported as positive values for successful calls, negative
	// otherwise.
	timings := make(chan result)

	go func(timingCh chan<- result, times int) {
		for i := 0; i < times; i++ {
			time.Sleep(config.Sleep)
			start := time.Now()
			res, err := failoverCall(clientPool, demoServerCall, &demo.LiffRequest{ID: int64(rand.Int())})
			stop := time.Now()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error calling Liff: %v\n", err)
				timingCh <- result{Success: false}
				continue
			}
			resp := res.(*demo.LiffResponse)
			timingCh <- result{
				NodeID:  resp.NodeID,
				Success: true,
				Time:    float64(stop.Sub(start)) / float64(time.Millisecond)}

		}
		wg.Done()
	}(timings, config.Repeats)

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
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if clientPool.Available() == 0 {
				fmt.Printf("Low Water Mark for pool. Refreshing it.")
				refreshPool(clientPool)
			}
		}
	}()

	success := 0
	received := 0
	go func() {
		for result := range timings {
			if !result.Success {
				continue
			}
			success++
			fmt.Printf("Time on %s: %f ms\n", result.NodeID, result.Time)

			nodes[result.NodeID]++
			totals[result.NodeID] += result.Time
			// Log call time to CSV
			fmt.Fprintf(csvFile, "%d,%s,%f\n", received, result.NodeID, result.Time)
			received++
		}
	}()

	wg.Wait()
	if config.PrintSummary {
		fmt.Println("=================================================")
		fmt.Printf("%d calls in total, %d successful, %d failed\n", config.Repeats*numWorkers, success, (numWorkers*config.Repeats)-success)
		for k, v := range nodes {
			fmt.Printf("%20s: %d calls - %f ms average\n", k, v, totals[k]/float64(v))
		}
	}
}

// No need to test the conversion since we're providing both lists but if you
// want to make *really* sure use the
func demoServerCall(ctx context.Context, conn *grpc.ClientConn, parameter interface{}) (interface{}, error) {
	// This isn't necessary but it's nice for debugging
	c := demo.NewDemoServiceClient(conn)
	p, ok := parameter.(*demo.LiffRequest)
	if !ok {
		panic("supplied parameter isn't LiffRequest")
	}
	return c.Liff(ctx, p)

	// The above could be a one liner like this:
	// return client.(demo.DemoServiceClient).Liff(ctx, parameter.(*demo.LiffRequest))
}

// The below will be moved to the clientfunk package later
type serverCall func(ctx context.Context, conn *grpc.ClientConn, parameter interface{}) (interface{}, error)

// Do repeated calls to different clients. If the first doesn't respond within
// 50 ms try the next with 100 ms timeout, then the final with 200 ms timeout.
// if all three fails return with an error.

func failoverCall(pool *clientfunk.ClientPool, call serverCall, param interface{}) (interface{}, error) {
	retCh := make(chan interface{})
	errCh := make(chan error)

	// This is the calling goroutine. It returns the error and response on the channels
	doCall := func(ctx context.Context, conn *grpc.ClientConn, pool *clientfunk.ClientPool, call serverCall, respCh chan<- interface{}, errCh chan<- error) {
		resp, err := call(ctx, conn, param)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}

	// Start with a 32 ms timeout and quadruple for each attempt (ie 32, 128, 512)
	timeout := time.Millisecond * 32
	// Keep on trying until either successful or we've tried three times
	for timeout < 1024*time.Millisecond {
		// Use the double timeout or we'd get an error right away if the first
		// client doesn't respond
		ctx, cancel := context.WithTimeout(context.Background(), timeout*2)
		defer cancel()

		conn, err := pool.Take(ctx)
		if err == context.DeadlineExceeded {
			fmt.Println("Unable to get connection. Going to sleep")
			time.Sleep(1 * time.Second)
			continue
		}
		if err != nil {
			// Unable to get a clientconn - should ideally block here.
			fmt.Printf("Unable to get connection: %v", err)
			return nil, err
		}
		go doCall(ctx, conn, pool, call, retCh, errCh)
		select {
		case resp := <-retCh:
			pool.Release(conn)

			// Great success. The client returned a response
			return resp, nil
		case err := <-errCh:
			code := status.Code(err)
			if code == codes.Unavailable {
				// Service is down, try another client
				pool.MarkUnhealthy(conn)
				break
			}
			if err == context.DeadlineExceeded {
				pool.MarkUnhealthy(conn) // Might be better to mark it unhealthy
				break
			}
			// Any other gRPC error is just passed along.
			pool.Release(conn) // Might be better to mark it unhealthy
			return nil, err
		case <-time.After(timeout):
			pool.Release(conn) // Might be better to mark it unhealthy

			// Next client
		}

		timeout *= 10
	}

	return nil, errors.New("all clients timed out")
}

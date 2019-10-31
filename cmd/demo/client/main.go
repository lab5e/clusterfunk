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

	"github.com/stalehd/clusterfunk/pkg/toolbox"

	"github.com/stalehd/clusterfunk/cmd/demo"
	"github.com/stalehd/clusterfunk/pkg/clientfunk"
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

const numWorkers = 3

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
	pool.Sync(endpoints)
}
func main() {
	clientPool := clientfunk.NewClientPool([]grpc.DialOption{grpc.WithInsecure()})
	refreshPool(clientPool)
	lowWatermark := clientPool.Size() - 1
	// Seed is quite important here
	rand.Seed(time.Now().UnixNano())

	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// Timings are reported as positive values for successful calls, negative
	// otherwise.
	timings := make(chan result)

	for worker := 0; worker < numWorkers; worker++ {
		go func(timingCh chan<- result, times int) {
			for i := 0; i < times; i++ {
				time.Sleep(config.Sleep)
				ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
				conn, err := clientPool.Take(ctx)
				done()
				if err != nil {
					continue
				}
				ctx, done = context.WithTimeout(context.Background(), 3*time.Second)
				liffClient := demo.NewDemoServiceClient(conn)
				start := time.Now()
				res, err := liffClient.Liff(ctx, &demo.LiffRequest{ID: int64(rand.Int())})
				stop := time.Now()
				done()
				if err != nil {
					code := status.Code(err)
					switch code {
					case codes.Unavailable /*, codes.DeadlineExceeded*/ :
						fmt.Printf("Connection %s might be unhealthy (deadline). Marking as unhealthy\n", conn.Target())
						clientPool.MarkUnhealthy(conn)
					default:
						fmt.Fprintf(os.Stderr, "Error calling Liff: %v (code=%v)\n", err, code)
						clientPool.Release(conn)
					}
					timingCh <- result{Success: false}
					continue
				}

				clientPool.Release(conn)

				timingCh <- result{
					NodeID:  res.NodeID,
					Success: true,
					Time:    float64(stop.Sub(start)) / float64(time.Millisecond)}

			}
			wg.Done()
		}(timings, config.Repeats)
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
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if clientPool.Size() < lowWatermark {
				refreshPool(clientPool)
			}
		}
	}()

	success := 0
	received := 0
	total := 0.0
	max := 0.0
	min := 99999999.9
	go func() {
		for result := range timings {
			received++
			fmt.Printf("\rReceived %d responses", received)
			if !result.Success {
				fmt.Printf("Error     : %f ms\n", result.Time)
				continue
			}
			success++
			total += result.Time
			if max < result.Time {
				max = result.Time
			}
			if min > result.Time {
				min = result.Time
			}
			nodes[result.NodeID]++
			totals[result.NodeID] += result.Time
			// Log call time to CSV
			fmt.Fprintf(csvFile, "%d,%s,%f\n", received, result.NodeID, result.Time)
		}
	}()

	wg.Wait()
	if config.PrintSummary {
		fmt.Println("=================================================")
		fmt.Printf("average: %6.2f   min: %6.2f  max: %6.2f\n", total/float64(success), min, max)
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

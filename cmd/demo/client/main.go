package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/stalehd/clusterfunk/cmd/demo"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s [endpoint] [command: liff, key, slow] [repeats, default 1]\n", os.Args[0])
		return
	}
	ep := os.Args[1]
	cmd := os.Args[2]

	repeats := 1
	if len(os.Args) > 3 {
		v, err := strconv.ParseInt(os.Args[3], 10, 32)
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
	var csvFile *os.File
	if repeats > 1 {
		csvFile, err := os.Create("timing.csv")
		if err != nil {
			fmt.Printf("Can't create file timing.csv: %v\n", err)
			return
		}
		fmt.Fprintf(csvFile, "Num,Node,Time\n")
	}
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
		switch cmd {
		case "liff":
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
		case "key":
			resp, err := client.MakeKeyPair(ctx, &demo.KeyPairRequest{ID: requestid})
			end = time.Now()
			if err != nil {
				fmt.Printf("Error calling MakeKeyPair: %v\n", err)
				if repeats == 1 {
					return
				}
			} else {
				nodeid = resp.NodeID
			}
		default:
			resp, err := client.Slow(ctx, &demo.SlowRequest{ID: requestid})
			end = time.Now()
			if err != nil {
				fmt.Printf("Error calling Slow: %v\n", err)
				if repeats == 1 {
					return
				}
			} else {
				nodeid = resp.NodeID
			}
		}
		end.Sub(start)
		callTime := float64(end.Sub(start)) / float64(time.Millisecond)
		if nodeid != "" {
			fmt.Printf("Time to call %s on %s: %f ms\n", cmd, nodeid, callTime)
			if repeats > 1 {
				nodes[nodeid]++
				totals[nodeid] += callTime
				// Log call time to CSV
				fmt.Fprintf(csvFile, "%d,%s,%f\n", i, nodeid, callTime)
			}
		}
	}
	if repeats > 1 {
		fmt.Println("=================================================")
		for k, v := range nodes {
			fmt.Printf("%20s: %d calls - %f ms average\n", k, v, totals[k]/float64(v))
		}
		csvFile.Close()
	}
}

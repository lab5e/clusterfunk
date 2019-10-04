package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/stalehd/clusterfunk/cmd/demo"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s [endpoint] [command: liff, key, slow]\n", os.Args[0])
		return
	}
	ep := os.Args[1]
	cmd := os.Args[2]

	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(ep, opts...)
	if err != nil {
		fmt.Printf("Error dialing to client: %v\n", err)
		return
	}
	defer conn.Close()
	client := demo.NewDemoServiceClient(conn)

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
			return
		}
		nodeid = resp.NodeID
		fmt.Println(resp.Definition)
	case "key":
		resp, err := client.MakeKeyPair(ctx, &demo.KeyPairRequest{ID: requestid})
		end = time.Now()
		if err != nil {
			fmt.Printf("Error calling MakeKeyPair: %v\n", err)
			return
		}
		nodeid = resp.NodeID
	default:
		resp, err := client.Slow(ctx, &demo.SlowRequest{ID: requestid})
		end = time.Now()
		if err != nil {
			fmt.Printf("Error calling Slow: %v\n", err)
			return
		}
		nodeid = resp.NodeID
	}
	end.Sub(start)
	if nodeid == "" {
		panic("no node id")
	}
	fmt.Printf("Time to call %s on %s: %f ms\n", cmd, nodeid, float64(end.Sub(start))/float64(time.Millisecond))
}

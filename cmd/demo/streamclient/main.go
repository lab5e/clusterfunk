package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/ExploratoryEngineering/clusterfunk/cmd/demo"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/clientfunk"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk"
	"github.com/alecthomas/kong"
)

type parameters struct {
	Endpoints   string              `kong:"help='Comma-separated list of endpoints to use. Will use zeroconf to find the parameters'"`
	ClusterName string              `kong:"help='Cluster name',default='clusterfunk'"`
	ZeroConf    bool                `kong:"help='ZeroConf lookups for cluster',default='true'"`
	Retry       bool                `kong:"help='Do a single retry for failed requests',default='true'"`
	Serf        funk.SerfParameters `kong:"embed,prefix='serf-'"`
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

	em, err := clientfunk.StartEndpointMonitor("", config.ClusterName, config.ZeroConf, config.Serf)
	if err != nil {
		fmt.Printf("Unable to start endpoint monitor: %v\n", err)
		return
	}
	defer em.Stop()

	em.WaitForEndpoints()

	grpcConnection, err := grpc.Dial("cluster:///ep.demo",
		grpc.WithInsecure(),
		grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))

	if err != nil {
		fmt.Println("Unable to get connection: ", err)
		return
	}
	defer grpcConnection.Close()

	lc := demo.NewDemoServiceClient(grpcConnection)
	ctx, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()

	fmt.Println("Client stream")
	cs, err := lc.ClientStreams(ctx)
	if err != nil {
		panic(err.Error())
	}
	for i := 0; i < 10; i++ {
		if err := cs.Send(&demo.Hello{
			Id:      int64(i),
			Message: "Hello",
		}); err != nil {
			panic(err.Error())
		}
		time.Sleep(1000 * time.Millisecond)
	}
	cs.CloseSend()

	fmt.Println("Wait...")
	time.Sleep(5 * time.Second)

	ctx2, done2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer done2()
	fmt.Println("Server streaming")
	ss, err := lc.ServerStreams(ctx2, &demo.Hello{Id: 99, Message: "Hello hello"})
	if err != nil {
		panic(err.Error())
	}
	finished := false
	for !finished {
		m, err := ss.Recv()
		if err != nil {
			finished = true
			continue
		}
		fmt.Printf("Got message from server: %v\n", m)
	}

	fmt.Println("Wait...")
	time.Sleep(5 * time.Second)

	fmt.Println("Both streaming")

	ctx3, done3 := context.WithTimeout(context.Background(), 20*time.Second)
	defer done3()

	bs, err := lc.BothStreams(ctx3)
	if err != nil {
		panic(err.Error())
	}
	go func() {
		for i := 0; i < 10; i++ {
			if err := bs.Send(&demo.Hello{
				Id:      int64(i),
				Message: "Hello",
			}); err != nil {
				panic(err.Error())
			}
			time.Sleep(1000 * time.Millisecond)
		}
		bs.CloseSend()
	}()
	for !finished {
		m, err := bs.Recv()
		if err != nil {
			finished = true
			continue
		}
		fmt.Printf("Got message from server: %v\n", m)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("Done")
}

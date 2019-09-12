package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stalehd/clattering/cluster"
)

func waitForTermination() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sig:
	}

}
func registerService(name string) {
	log.Println("Registering service")
	reg := cluster.NewZeroconfRegistry(name)

	nodeName := fmt.Sprintf("node_%08x", rand.Int())
	if err := reg.Register(nodeName, int(rand.Int31n(31000))+1001); err != nil {
		log.Printf("Error registering service: %v", err)
		return
	}
	waitForTermination()

	reg.Shutdown()
}

func browseService(name string) {
	log.Println("Browse service")
	reg := cluster.NewZeroconfRegistry(name)
	results, err := reg.Resolve(2 * time.Second)
	if err != nil {
		log.Printf("Error browsing for zeroconf: %v", err)
		return
	}

	for i := range results {
		log.Printf("Item: %s", results[i])
	}
}

func main() {
	register := flag.Bool("register", true, "Register in Zeroconf")
	cluster := flag.String("name", "demo-cluster", "Name of cluster")
	flag.Parse()
	if *register {
		registerService(*cluster)
		return
	}
	browseService(*cluster)
}

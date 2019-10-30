package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/stalehd/clusterfunk/toolbox"
)

func registerService(name string) {
	log.Println("Registering service")
	reg := toolbox.NewZeroconfRegistry(name)

	if err := reg.Register("node", fmt.Sprintf("%08x", rand.Int()), int(rand.Int31n(31000))+1001); err != nil {
		log.Printf("Error registering service: %v", err)
		return
	}
	toolbox.WaitForCtrlC()

	reg.Shutdown()
}

func browseService(name string) {
	log.Println("Browse service")
	reg := toolbox.NewZeroconfRegistry(name)
	results, err := reg.Resolve("node", 2*time.Second)
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

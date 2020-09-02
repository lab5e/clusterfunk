package main

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/lab5e/clusterfunk/pkg/toolbox"
	gotoolbox "github.com/lab5e/gotoolbox/toolbox"
)

func registerService(name string) {
	log.Println("Registering service")
	reg := toolbox.NewZeroconfRegistry(name)

	if err := reg.Register("node", fmt.Sprintf("%08x", rand.Int()), int(rand.Int31n(31000))+1001); err != nil {
		log.Printf("Error registering service: %v", err)
		return
	}
	gotoolbox.WaitForSignal()

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

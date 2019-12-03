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
	"time"

	"github.com/stalehd/clusterfunk/pkg/toolbox"

	"github.com/stalehd/clusterfunk/pkg/funk"
)

var eps = []string{
	"127.0.0.1:9990",
	"127.0.0.1:9991",
	"127.0.0.1:9992",
	"127.0.0.1:9993",
	"127.0.0.1:9994",
	"127.0.0.1:9995",
	"127.0.0.1:9996",
	"127.0.0.1:9997",
	"127.0.0.1:9998",
	"127.0.0.1:9999",
}

func main() {

	client := flag.Bool("client", false, "Client mode")
	server := flag.Bool("server", false, "Server mode")
	flag.Parse()

	if !(*client != *server) {
		fmt.Println("Must specify client or server")
		return
	}

	fmt.Println("Ctrl+C to stop")
	if *client {
		for _, v := range eps {
			c := funk.NewLivenessClient(v)
			defer c.Stop()
		}
	}
	if *server {
		s := funk.NewLivenessChecker(10*time.Millisecond, 3)
		for i, v := range eps {
			s.Add(fmt.Sprintf("client%02d", i), v)
		}

		go func() {
			for k := range s.AliveEvents() {
				fmt.Printf("%s is alive\n", k)
			}
		}()
		go func() {
			for k := range s.DeadEvents() {
				fmt.Printf("%s died\n", k)
			}
		}()
	}
	toolbox.WaitForCtrlC()
	fmt.Println("Stopping...")
}

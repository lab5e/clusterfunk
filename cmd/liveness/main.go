package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/stalehd/clusterfunk/toolbox"

	"github.com/stalehd/clusterfunk/funk"
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

	if *client {
		for _, v := range eps {
			c := funk.NewLivenessClient(v)
			defer c.Stop()
		}
		fmt.Println("Ctrl+C to stop")
		toolbox.WaitForCtrlC()
		return
	}
	if *server {
		s := funk.NewLivenessChecker(10*time.Millisecond, 3)
		for i, v := range eps {
			s.Add(fmt.Sprintf("client%02d", i), v)
		}

		dead := 0
		for k := range s.DeadEvents() {
			fmt.Printf("%s died\n", k)
			dead++
			if dead == len(eps) {
				break
			}
		}
		return
	}
}

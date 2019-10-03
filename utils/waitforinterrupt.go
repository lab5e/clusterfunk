package utils

import (
	"os"
	"os/signal"
)

// WaitForCtrlC waits for an interrupt signal.
func WaitForCtrlC() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	for {
		select {
		case <-terminate:
			return
		}
	}
}

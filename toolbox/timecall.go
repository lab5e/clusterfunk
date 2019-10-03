package toolbox

import (
	"log"
	"time"
)

// TimeCall times the call and prints out the execution time in milliseconds in
// the go standard log.
func TimeCall(call func(), description string) {
	start := time.Now()
	call()
	diff := time.Now().Sub(start)
	log.Printf("%s took %f ms to execute", description, float64(diff)/float64(time.Millisecond))
}

package toolbox

import (
	log "github.com/sirupsen/logrus"

	"time"
)

// TimeCall times the call and prints out the execution time in milliseconds in
// the go standard log.
func TimeCall(call func(), description string) {
	start := time.Now()
	call()
	diff := time.Since(start)
	log.Infof("%s took %f ms to execute", description, float64(diff)/float64(time.Millisecond))
}

package cluster

import "log"

// this is a temporary implementation of the hashicorp logger interface
type clusterLogger struct {
	Logger *log.Logger
}

type muteWriter struct {
}

func (m *muteWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func newClusterLogger(prefix string) *clusterLogger {

	return &clusterLogger{
		Logger: log.New(&muteWriter{}, prefix, log.LstdFlags),
	}
}

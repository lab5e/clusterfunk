module github.com/stalehd/clusterfunk

go 1.12

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/golang/protobuf v1.3.2
	github.com/grandcat/zeroconf v0.0.0-20190424104450-85eadb44205c
	github.com/hashicorp/raft v1.1.1
	github.com/hashicorp/raft-boltdb v0.0.0-20190605210249-ef2e128ed477
	github.com/hashicorp/serf v0.8.4
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	google.golang.org/grpc v1.24.0
)

// Using a non-pushed version of Raft here.
//replace github.com/hashicorp/raft v1.1.1 => github.com/stalehd/raft v1.1.2-0.20190926134415-e3913cefd917

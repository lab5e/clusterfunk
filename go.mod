module github.com/stalehd/clusterfunk

go 1.12

require (
	github.com/ExploratoryEngineering/params v1.0.0
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.1
	github.com/grandcat/zeroconf v0.0.0-20190424104450-85eadb44205c
	github.com/hashicorp/raft v1.1.0
	github.com/hashicorp/raft-boltdb v0.0.0-20190605210249-ef2e128ed477
	github.com/hashicorp/serf v0.8.4
	github.com/kr/pretty v0.1.0 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529 // indirect
	google.golang.org/grpc v1.24.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

// Using a non-pushed version of Raft here.
//replace github.com/hashicorp/raft v1.1.1 => github.com/stalehd/raft v1.1.2-0.20190926134415-e3913cefd917

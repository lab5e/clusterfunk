module github.com/stalehd/clusterfunk

go 1.12

require (
	github.com/ExploratoryEngineering/logging v0.0.0-20181106085733-dcb8702a004e // indirect
	github.com/ExploratoryEngineering/params v1.0.0
	github.com/ExploratoryEngineering/rest v0.0.0-20181001125504-5b79b712352a
	github.com/aclements/go-moremath v0.0.0-20190830160640-d16893ddf098
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.1
	github.com/grandcat/zeroconf v0.0.0-20190424104450-85eadb44205c
	github.com/hashicorp/raft v1.1.0
	github.com/hashicorp/raft-boltdb v0.0.0-20190605210249-ef2e128ed477
	github.com/hashicorp/serf v0.8.4
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-runewidth v0.0.6 // indirect
	github.com/nsf/termbox-go v0.0.0-20190817171036-93860e161317 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749 // indirect
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529 // indirect
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.org/x/sys v0.0.0-20191110163157-d32e6e3b99c4 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/genproto v0.0.0-20190819201941-24fa4b261c55 // indirect
	google.golang.org/grpc v1.24.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

// Using a non-pushed version of Raft here.
//replace github.com/hashicorp/raft v1.1.1 => github.com/stalehd/raft v1.1.2-0.20190926134415-e3913cefd917

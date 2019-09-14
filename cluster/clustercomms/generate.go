package clustercomms

//go:generate protoc -I=../../protobuf --go_out=plugins=grpc:. ../../protobuf/cluster.proto

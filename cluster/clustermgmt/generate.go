package clustermgmt

//go:generate protoc -I=../../protobuf --go_out=plugins=grpc:. ../../protobuf/leader_management.proto
//go:generate protoc -I=../../protobuf --go_out=plugins=grpc:. ../../protobuf/node_management.proto

all: test bins

clean:
	go clean

bins:
	cd cmd/ctrlc && go build -o ../../bin/ctrlc
	cd cmd/demo/server && go build -o ../../../bin/demo
	cd cmd/demo/client && go build -o ../../../bin/client
	cd cmd/raft && go build -o ../../bin/raft

test:
	go test ./... --cover


generate:
	go generate ./...


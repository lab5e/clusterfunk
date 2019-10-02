all: test bins

clean:
	go clean

bins:
	cd cmd/ctrlc && go build -o ../../bin/ctrlc
	cd cmd/demo && go build -o ../../bin/demo
	cd cmd/raft && go build -o ../../bin/raft

test:
	go test ./... --cover


generate:
	go generate ./...


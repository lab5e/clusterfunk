all: generate test bins

clean:
	go clean

bins:
	cd cmd/ctrlc && go build -o ../../bin/ctrlc
	cd cmd/demo && go build -o ../../bin/demo

test:
	go test ./... --cover


generate:
	go generate ./...


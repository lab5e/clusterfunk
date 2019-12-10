all: nolint lint

nolint: test bins

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

lint:
	golint ./...
	go vet ./...
	staticcheck ./...
	revive ./...
	golangci-lint run

# The linux build uses the docker core images to build. If you are running on Linux you might as well just
# build it directly (use go build -installsuffix cgo -o ...)
# Make sure you have the latest version locally. If you have an older version it will give you complation errors.
linux:
	docker run --rm -it -v ${GOPATH}:/go -v $(CURDIR):/clusterfunk -w /clusterfunk/cmd dockercore/golang-cross:latest sh -c '\
	   cd demo/server && echo Demo server...     && GOOS=linux GOARCH=amd64 go build -installsuffix cgo -o ../../../bin/demo.linux  \
	&& cd ../client && echo Demo client...       && GOOS=linux GOARCH=amd64 go build -installsuffix cgo -o ../../../bin/client.linux \
	&& cd ../../ctrlc && echo Ctrlc...           && GOOS=linux GOARCH=amd64 go build -installsuffix cgo -o ../../bin/ctrl.linux'


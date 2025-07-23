.PHONY: all build test lint fmt clean

all: build

build:
	go mod tidy
	go build -ldflags="-s -w"

test:
	go test ./... -v -race
	(cd test; npm test)

lint:
	$(GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...
	$(GOPATH)/bin/goimports -w .

clean:
	rm -f mobilecli coverage.out

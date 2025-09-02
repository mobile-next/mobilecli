.PHONY: all build test test-cover lint fmt clean

all: build

build:
	go mod tidy
	go build -ldflags="-s -w"

test:
	go test ./... -v -race

test-cover: build
	go test ./... -v -race -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...
	$(shell go env GOPATH)/bin/goimports -w .

clean:
	rm -f mobilecli coverage.out coverage.html

.PHONY: all build test test-cover lint fmt clean

all: build

build:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w"

build-cover:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w" -cover

test:
	go test ./... -v -race

test-cover: build-cover
	go test ./... -v -race -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-e2e: build-cover
	rm -rf test/coverage
	(cd test && npm run test-simulator)
	go tool covdata textfmt -i=test/coverage -o cover.out
	go tool cover -func=cover.out

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...
	$(shell go env GOPATH)/bin/goimports -w .

clean:
	rm -f mobilecli coverage.out coverage.html

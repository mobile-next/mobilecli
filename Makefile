.PHONY: all build test lint fmt clean install-lint-deps

all: build

build:
	go mod tidy
	go build

test:
	go test ./... -v -race

lint:
	$(GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...
	$(GOPATH)/bin/goimports -w .

clean:
	rm -f mobilectl coverage.out

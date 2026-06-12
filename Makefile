.PHONY: all build agents test test-cover lint fmt clean docs

all: build

agents:
	$(MAKE) -C agents/android all
	$(MAKE) -C agents/ios all

build: agents
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w"

build-cover: agents
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w" -cover

test:
	go test ./... -v -race

test-cover: build-cover
	go test ./... -v -race -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-e2e: build-cover
	rm -rf test/coverage
	(cd test && npm run test:simulator)
	go tool covdata textfmt -i=test/coverage -o cover.out
	go tool cover -func=cover.out

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

docs:
	npx https://github.com/gmegidish/jagger -f markdown -o docs/openrpc.md docs/openrpc.json

fmt:
	go fmt ./...
	$(shell go env GOPATH)/bin/goimports -w .

clean:
	$(MAKE) -C agents/android clean
	$(MAKE) -C agents/ios clean
	rm -f mobilecli coverage.out coverage.html


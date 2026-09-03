.PHONY: all build agents test test-cover lint fmt clean docs vulncheck

all: build

agents:
	$(MAKE) -C agents/android all
	$(MAKE) -C agents/ios all

build: agents
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w"

build-cover: agents
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w" -cover -covermode=atomic

test:
	go test ./... -v -race

test-cover: build-cover
	go test ./... -v -race -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-e2e: build-cover
	rm -rf test/coverage
	mkdir -p test/coverage
	go test ./... -v -race -covermode=atomic -args -test.gocoverdir=$(CURDIR)/test/coverage
	(cd test && npm run test:server)
	(cd test && npm run test:daemon)
	# (cd test && npm run test:ios)
	(cd test && npm run test:simulator)
	(cd test && npm run test:android)
	(cd test && npm run test:emulator)
	go tool covdata textfmt -i=test/coverage -o coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

vulncheck:
	$(shell go env GOPATH)/bin/govulncheck ./...

docs:
	npx https://github.com/gmegidish/jagger -f markdown -o docs/openrpc.md docs/openrpc.json

fmt:
	go fmt ./...
	$(shell go env GOPATH)/bin/goimports -w .

clean:
	$(MAKE) -C agents/android clean
	$(MAKE) -C agents/ios clean
	rm -f mobilecli coverage.out coverage.html


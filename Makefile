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
	# device commands run inside the daemon, so its counters are the bulk of the
	# coverage. stop any daemon already running: the suites spawn a fresh one that
	# inherits GOCOVERDIR, and a -cover binary only writes counters when it exits.
	./mobilecli daemon stop
	go test ./... -v -race -covermode=atomic -args -test.gocoverdir=$(CURDIR)/test/coverage
	# one shell with a trap, so a failing suite still stops the daemon and flushes
	# its counters into test/coverage before they are read back
	set -e; \
	trap 'GOCOVERDIR=$(CURDIR)/test/coverage ./mobilecli daemon stop >/dev/null 2>&1 || true' EXIT; \
	(cd test && npm run test:server); \
	(cd test && npm run test:daemon); \
	(cd test && npm run test:ios-simulator); \
	(cd test && npm run test:android); \
	(cd test && npm run test:emulator)
	go tool covdata textfmt -i=test/coverage -o coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

lint:
	$(MAKE) -C agents/android lint
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


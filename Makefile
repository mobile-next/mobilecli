.PHONY: all build test test-cover lint fmt clean

COVERAGE_DIR := $(CURDIR)/coverage
COVERAGE_UNIT := $(COVERAGE_DIR)/unit
COVERAGE_E2E := $(CURDIR)/test/coverage
COVERAGE_MERGED := $(COVERAGE_DIR)/merged

all: build

build:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w"

build-cover:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w" -cover

test:
	go test ./... -v -race

test-cover:
	@rm -rf $(COVERAGE_UNIT)
	@mkdir -p $(COVERAGE_UNIT)
	go test -cover ./... -args -test.gocoverdir=$(COVERAGE_UNIT)

test-e2e: build-cover
	@rm -rf $(COVERAGE_E2E)
	(cd test && npm test)

coverage: test-cover test-e2e
	@rm -rf $(COVERAGE_MERGED)
	@mkdir -p $(COVERAGE_MERGED)
	go tool covdata merge -i=$(COVERAGE_UNIT),$(COVERAGE_E2E) -o=$(COVERAGE_MERGED)
	go tool covdata textfmt -i=$(COVERAGE_MERGED) -o=$(COVERAGE_DIR)/coverage.out
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o=$(COVERAGE_DIR)/coverage.html
	@echo ""
	@echo "=== Coverage by package ==="
	@go tool covdata percent -i=$(COVERAGE_MERGED)
	@echo ""
	@echo "=== Total ==="
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total
	@echo ""
	@echo "HTML report: $(COVERAGE_DIR)/coverage.html"

lint:
	$(shell go env GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...
	$(shell go env GOPATH)/bin/goimports -w .

clean:
	rm -f mobilecli coverage.out coverage.html
	rm -rf coverage test/coverage

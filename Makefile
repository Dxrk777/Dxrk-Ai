.PHONY: all build test lint vet bench clean docker-up docker-down dev help

BINARY=dxrk
CMD=./cmd/dxrk
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION)"

all: fmt tidy lint vet test build

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test -count=1 -race -shuffle=on ./...

test-cover:
	go test -count=1 -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

bench:
	go test -bench=. -benchtime=100ms -count=3 ./... | tee bench.txt

bench-compare:
	@echo "=== Benchmark Comparison ==="
	@go test -bench=. -benchtime=100ms -count=1 ./internal/router/... 2>&1 | grep "Benchmark"
	@go test -bench=. -benchtime=100ms -count=1 ./internal/rag/... 2>&1 | grep "Benchmark"
	@go test -bench=. -benchtime=100ms -count=1 ./internal/pipeline/... 2>&1 | grep "Benchmark"
	@go test -bench=. -benchtime=100ms -count=1 ./internal/sandbox/... 2>&1 | grep "Benchmark"
	@go test -bench=. -benchtime=100ms -count=1 ./internal/vault/... 2>&1 | grep "Benchmark"
	@go test -bench=. -benchtime=100ms -count=1 ./internal/observe/... 2>&1 | grep "Benchmark"

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

docker-build:
	docker build -t dxrk:$(VERSION) .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

dev:
	go run $(CMD)

install:
	go install $(CMD)

clean:
	rm -f $(BINARY) coverage.out coverage.html bench.txt

help:
	@echo "Dxrk.ai — Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all          lint + test + build (default)"
	@echo "  build        compile binary"
	@echo "  test         run all tests with race detector"
	@echo "  test-cover   run tests with coverage report"
	@echo "  bench        run all benchmarks"
	@echo "  lint         run golangci-lint"
	@echo "  vet          run go vet"
	@echo "  fmt          format all Go files"
	@echo "  tidy         go mod tidy"
	@echo "  docker-build build Docker image"
	@echo "  docker-up    start all services"
	@echo "  docker-down  stop all services"
	@echo "  dev          run locally"
	@echo "  clean        remove artifacts"

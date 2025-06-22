.PHONY: build

all: lint build

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

build:
	@go build -o build/pdns-sensor ./cmd/pdns-sensor/*.go

tools:
	@echo "Running tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.1.6

.PHONY: build build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-all
VERSION ?= $(shell cat cmd/pdns-sensor/VERSION)

all: lint test build-all

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

build-linux-amd64:
	@echo "Building for Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build -o build/pdns-sensor-linux-amd64 ./cmd/pdns-sensor/*.go

build-linux-arm64:
	@echo "Building for Linux arm64..."
	@GOOS=linux GOARCH=arm64 go build -o build/pdns-sensor-linux-arm64 ./cmd/pdns-sensor/*.go

build-darwin-amd64:
	@echo "Building for macOS amd64..."
	@GOOS=darwin GOARCH=amd64 go build -o build/pdns-sensor-darwin-amd64 ./cmd/pdns-sensor/*.go

build-darwin-arm64:
	@echo "Building for macOS arm64 (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -o build/pdns-sensor-darwin-arm64 ./cmd/pdns-sensor/*.go

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64
	@echo "All cross-platform builds complete!"

tools:
	@echo "Running tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.3.1

tag:
	@echo "Tagging the current version..."
	git tag -a "v$(VERSION)" -m "Release version $(VERSION)"; \
	git push origin "v$(VERSION)"

test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=build/coverage.out ./...
	@go tool cover -html=build/coverage.out -o build/coverage.html

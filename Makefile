.PHONY: build

all: build

build:
	@go build -o build/pdns-sensor ./cmd/pdns-sensor/*.go

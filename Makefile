.PHONY: build run clean test install

BINARY_NAME=remotty
VERSION?=dev

build:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY_NAME)

clean:
	go clean
	rm -f $(BINARY_NAME)

test:
	go test -v ./...


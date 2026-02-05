.PHONY: build run clean test install

BINARY_NAME=remotty

build:
	go mod tidy
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME)

clean:
	go clean
	rm -f $(BINARY_NAME)

test:
	go test -v ./...


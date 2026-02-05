.PHONY: build run clean test install vulncheck

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

vulncheck:
	@test -f $(shell go env GOPATH)/bin/govulncheck || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	$(shell go env GOPATH)/bin/govulncheck ./... || true


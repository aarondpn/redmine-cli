BINARY_NAME=redmine
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/

install:
	go install $(LDFLAGS) .

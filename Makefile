.PHONY: all build test fmt install clean

BINARY := ws
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

all: fmt test build

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/ws

test:
	go test -race -count=1 ./...

fmt:
	go fmt ./...

install: build
	@cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	 cp bin/$(BINARY) $(HOME)/go/bin/$(BINARY) 2>/dev/null || \
	 sudo cp bin/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed $(BINARY)"

clean:
	rm -rf bin/

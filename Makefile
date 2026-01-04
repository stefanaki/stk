# stk - Stacked Branches CLI Tool

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -ldflags "-X github.com/gstefan/stk/internal/cli.Version=$(VERSION) \
                      -X github.com/gstefan/stk/internal/cli.Commit=$(COMMIT) \
                      -X github.com/gstefan/stk/internal/cli.Date=$(DATE)"

.PHONY: all build install clean test lint

all: build

build:
	go build $(LDFLAGS) -o stk ./cmd/stk

install:
	go install $(LDFLAGS) ./cmd/stk

clean:
	rm -f stk
	rm -rf dist/

test:
	go test -v ./...

lint:
	golangci-lint run

# Cross-compilation targets
.PHONY: build-all build-linux build-darwin build-windows

build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/stk-linux-amd64 ./cmd/stk
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/stk-linux-arm64 ./cmd/stk

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/stk-darwin-amd64 ./cmd/stk
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/stk-darwin-arm64 ./cmd/stk

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/stk-windows-amd64.exe ./cmd/stk

# Development helpers
.PHONY: run dev

run: build
	./stk $(ARGS)

dev:
	go run ./cmd/stk $(ARGS)

# Completion scripts
.PHONY: completion-bash completion-zsh completion-fish

completion-bash:
	./stk completion bash > completions/stk.bash

completion-zsh:
	./stk completion zsh > completions/_stk

completion-fish:
	./stk completion fish > completions/stk.fish

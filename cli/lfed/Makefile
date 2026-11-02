GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

# VERSION MUST be semver: pluginmgr.CheckHostVersion parses it. No release
# tags exist yet, so the commit hash goes in build metadata.
VERSION ?= 0.1.0-dev+$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
LDFLAGS := -X github.com/LF-Certification/p3-forge/cli/lfed/internal/build.Version=$(VERSION)

.PHONY: setup build test lint install-local clean

setup:
	go mod download

build:
	go build -ldflags '$(LDFLAGS)' -o bin/lfed ./cmd/lfed

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l cmd internal sdk)" || { gofmt -l cmd internal sdk; exit 1; }
	go tool govulncheck ./...

# Build lfed into $GOBIN.
install-local:
	go build -ldflags '$(LDFLAGS)' -o $(GOBIN)/lfed ./cmd/lfed

clean:
	rm -rf bin

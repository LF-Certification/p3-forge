GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

# VERSION MUST be semver: pluginmgr.CheckLfedVersion parses it. Local builds
# carry the commit hash as build metadata. Release builds get the tag version
# from .goreleaser.yaml.
VERSION ?= 0.1.0-dev+$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
LDFLAGS := -X github.com/LF-Certification/p3-forge/cli/lfed/internal/build.Version=$(VERSION)

.PHONY: setup build test lint install-local release-snapshot clean

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

# Build the release archives into dist/ without a tag or a publish step.
release-snapshot:
	goreleaser release --snapshot --clean --skip=publish

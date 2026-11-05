GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

VERSION ?= 0.1.0-dev+$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
LDFLAGS := -X github.com/LF-Certification/p3-forge/cli/lfed/internal/build.Version=$(VERSION)

.PHONY: setup build test lint install-local release-snapshot clean

setup:
	go mod download

GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags '-s -w $(LDFLAGS)'

GOSEC := github.com/securego/gosec/v2/cmd/gosec@v2.30.0

build:
	$(GOBUILD) -o bin/lfed ./cmd/lfed

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l cmd internal sdk)" || { gofmt -l cmd internal sdk; exit 1; }
	go tool staticcheck ./...
	go run $(GOSEC) -quiet -exclude-generated -exclude=G104,G204,G304,G702 ./...
	go tool govulncheck ./...

install-local:
	$(GOBUILD) -o $(GOBIN)/lfed ./cmd/lfed

clean:
	rm -rf bin

release-snapshot:
	goreleaser release --snapshot --clean --skip=publish,sign

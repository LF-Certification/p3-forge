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

# Same build settings as .goreleaser.yaml: static, no local paths, stripped.
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags '-s -w $(LDFLAGS)'

# gosec is pinned. Excluded rules:
#   G204  subprocess with variable arguments: the host runs plugin binaries.
#   G702  command injection by taint analysis: the same exec as G204, and
#         sdk/auth re-runs the host from LFED_REEXEC.
#   G304  file path from a variable: the host reads plugin and config paths.
#   G104  unchecked errors: go vet and staticcheck report these.
# Other false positives carry a `#nosec <rule> -- <reason>` comment.
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

# Build lfed into $GOBIN.
install-local:
	$(GOBUILD) -o $(GOBIN)/lfed ./cmd/lfed

clean:
	rm -rf bin

# Build the release archives into dist/ without a tag, a signature or a
# publish step.
release-snapshot:
	goreleaser release --snapshot --clean --skip=publish,sign

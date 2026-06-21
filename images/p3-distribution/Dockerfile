# Derived build of distribution/distribution (the registry / pull-through cache).
#
# Pinned to an unreleased main commit that fixes the proxy writing cached
# content under the client's HTTP request context: a client disconnect mid-pull
# could abort the background cache write and leave a truncated object in the
# cache.
#
# Fix commit: fix(registry/proxy): do not re-use http request context for
# background writes (2026-03-27). There is no upstream release containing it yet
# (latest is v3.1.1), hence this from-source build.
#
# Drop-in for ghcr.io/distribution/distribution: same ENTRYPOINT, port and
# default config path. The binary reads its config from a mounted file, so only
# the registry binary matters here.

ARG GO_VERSION=1.25.9
ARG ALPINE_VERSION=3.23
ARG DISTRIBUTION_COMMIT=e0bac4837514ad7e3339fdbd3fe2fbb67a81b899

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build
ARG TARGETOS
ARG TARGETARCH
ARG DISTRIBUTION_COMMIT
ARG PKG=github.com/distribution/distribution/v3
ENV CGO_ENABLED=0
RUN apk add --no-cache git
WORKDIR /src
RUN git clone https://github.com/distribution/distribution.git . \
    && git checkout "${DISTRIBUTION_COMMIT}"
# Cross-compile natively on the build platform (pure Go, no emulation needed).
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags "-s -w -X ${PKG}/version.version=3.1.1-ptcfix-${DISTRIBUTION_COMMIT} -X ${PKG}/version.revision=${DISTRIBUTION_COMMIT} -X ${PKG}/version.mainpkg=${PKG}" \
    -o /out/registry ./cmd/registry

FROM alpine:${ALPINE_VERSION}

# OCI labels for package linking and metadata
LABEL org.opencontainers.image.source="https://github.com/lf-certification/sandbox-images"
LABEL org.opencontainers.image.title="p3-distribution"
LABEL org.opencontainers.image.description="distribution/distribution registry built from a pinned proxy-fix commit"

RUN apk add --no-cache ca-certificates
COPY --from=build /out/registry /bin/registry
COPY --from=build /src/cmd/registry/config-dev.yml /etc/distribution/config.yml
VOLUME ["/var/lib/registry"]
EXPOSE 5000
ENTRYPOINT ["registry"]
CMD ["serve", "/etc/distribution/config.yml"]

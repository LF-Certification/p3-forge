# Build stage
FROM node:18-alpine AS builder

# Install make and other build dependencies
RUN apk add --no-cache make bash coreutils

# Set working directory
WORKDIR /build

# Copy source files
COPY . .

# Build all components
RUN make build

# Final stage
FROM caddy:2-alpine

# Create necessary directories
RUN mkdir -p /usr/share/caddy/dist /usr/share/caddy/config

# Copy built assets from build stage
COPY --from=builder /build/lab-ui/dist/ /usr/share/caddy/dist/

# Copy configuration files
COPY dev/config.json /usr/share/caddy/config.json
COPY dev/Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
WORKDIR /usr/share/caddy

# syntax=docker/dockerfile:1

FROM golang:1.25 AS builder

WORKDIR /build

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Transfer source code
COPY .git/refs/heads/main ./commitID
COPY *.go ./
COPY src ./src

# Build
ENV CGO_ENABLED=0
RUN commit_hash=$(cat commitID | cut -c1-7) && \
    go build -ldflags "-X github.com/birabittoh/disgord/src/globals.CommitID=$commit_hash" -o /dist/disgord

# Install playwright firefox via npm (Azure CDN for playwright driver is unreliable)
RUN apt-get update && apt-get install -y --no-install-recommends nodejs npm && \
    npm install -g playwright-core@1.60.0 && \
    DRIVER_DIR="/root/.cache/ms-playwright-go/1.60.0" && \
    mkdir -p "$DRIVER_DIR" && \
    cp -r "$(npm root -g)/playwright-core" "$DRIVER_DIR/package" && \
    ln -sf /usr/bin/node "$DRIVER_DIR/node" && \
    node "$DRIVER_DIR/package/cli.js" install firefox && \
    rm -rf /var/lib/apt/lists/*

# Test
FROM builder AS run-test-stage
RUN go test -v ./...

FROM debian:bookworm-slim AS build-release-stage

# Firefox runtime deps + ffmpeg + nodejs (required by playwright driver)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    nodejs \
    libgtk-3-0 libdbus-glib-1-2 libxt6 libasound2 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY templates ./templates
COPY --from=builder /dist .
COPY --from=builder /root/.cache/ms-playwright /root/.cache/ms-playwright
COPY --from=builder /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go

ENV PLAYWRIGHT_NODEJS_PATH=/usr/bin/node

ENTRYPOINT ["./disgord"]

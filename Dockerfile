# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# Build stage
# checkov:skip=CKV_DOCKER_7: The local builder uses the current Chainguard Go image.
# hadolint ignore=DL3007
FROM cgr.dev/chainguard/go:latest AS builder

ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with version info
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

RUN go build -o /go/bin/survey-api -trimpath \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    github.com/linuxfoundation/lfx-v2-survey-service/cmd/survey-api

# Final stage
FROM cgr.dev/chainguard/static:latest@sha256:d6a97eb401cbc7c6d48be76ad81d7899b94303580859d396b52b67bc84ea7345

# Use non-root user. Numeric UID of the Chainguard nonroot user, matching the
# chart's runAsUser.
USER 65532

# Copy binary from builder
COPY --from=builder /go/bin/survey-api /cmd/survey-api

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/cmd/survey-api", "-health-check"]

ENTRYPOINT ["/cmd/survey-api"]

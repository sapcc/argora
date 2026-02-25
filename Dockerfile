# SPDX-FileCopyrightText: 2025 SAP SE
#
# SPDX-License-Identifier: Apache-2.0

ARG TARGET_GO_VERSION=1.26.0

# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:${TARGET_GO_VERSION} AS builder
ARG TARGETOS TARGETARCH
ARG BININFO_BUILD_DATE BININFO_COMMIT_HASH BININFO_VERSION

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/manager/main.go cmd/manager/main.go
COPY api/ api/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags "-s -w -X github.com/sapcc/go-api-declarations/bininfo.binName=manager -X github.com/sapcc/go-api-declarations/bininfo.version=${BININFO_VERSION} -X github.com/sapcc/go-api-declarations/bininfo.commit=${BININFO_COMMIT_HASH} -X github.com/sapcc/go-api-declarations/bininfo.buildDate=${BININFO_BUILD_DATE}" -a -o manager cmd/manager/main.go


# Debug image (with Delve, non-distroless)
FROM --platform=$BUILDPLATFORM golang:${TARGET_GO_VERSION} AS debug
ARG BININFO_BUILD_DATE BININFO_COMMIT_HASH BININFO_VERSION
LABEL source_repository="https://github.com/sapcc/argora" \
  org.opencontainers.image.url="https://github.com/sapcc/argora" \
  org.opencontainers.image.created=${BININFO_BUILD_DATE} \
  org.opencontainers.image.revision=${BININFO_COMMIT_HASH} \
  org.opencontainers.image.version=${BININFO_VERSION}
ARG TARGETOS TARGETARCH
ARG BININFO_BUILD_DATE BININFO_COMMIT_HASH BININFO_VERSION
WORKDIR /

COPY --from=builder /workspace .
RUN GOBIN=/go/bin go install github.com/go-delve/delve/cmd/dlv@latest

RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags "-X github.com/sapcc/go-api-declarations/bininfo.binName=manager -X github.com/sapcc/go-api-declarations/bininfo.version=${BININFO_VERSION} -X github.com/sapcc/go-api-declarations/bininfo.commit=${BININFO_COMMIT_HASH} -X github.com/sapcc/go-api-declarations/bininfo.buildDate=${BININFO_BUILD_DATE}" -gcflags "-N -l" -o manager cmd/manager/main.go
USER 65532:65532
ENTRYPOINT ["dlv", "exec", "/manager", "--headless", "--listen=:3000", "--accept-multiclient", "--continue", "--"]

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS manager
ARG BININFO_BUILD_DATE BININFO_COMMIT_HASH BININFO_VERSION

LABEL source_repository="https://github.com/sapcc/argora" \
  org.opencontainers.image.url="https://github.com/sapcc/argora" \
  org.opencontainers.image.created=${BININFO_BUILD_DATE} \
  org.opencontainers.image.revision=${BININFO_COMMIT_HASH} \
  org.opencontainers.image.version=${BININFO_VERSION}

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]

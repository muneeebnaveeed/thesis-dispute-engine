# syntax=docker/dockerfile:1
# Multi-stage: build with the pinned Go toolchain, run from distroless as a
# non-root user. GO_VERSION must match mise.toml; scripts/check-runtime-versions.sh
# fails CI on drift, and CI passes the value in from mise.toml.
ARG GO_VERSION=1.27.1

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
# Dependencies first so the module download layer is reused across source changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/telemetry.Version=${VERSION}" \
      -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot
# One image, three entrypoints: /api serves, /migrate and /seed run to completion as the schema owner.
COPY --from=build /out/api /out/migrate /out/seed /
EXPOSE 8090
USER nonroot:nonroot
ENTRYPOINT ["/api"]

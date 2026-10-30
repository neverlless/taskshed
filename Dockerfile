# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/taskshed ./cmd/server

# distroless has no shell, so the data dir is prepared here
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot

ARG VERSION=dev
LABEL org.opencontainers.image.title="taskshed" \
      org.opencontainers.image.description="Centralized view of scheduled tasks across distributed systems" \
      org.opencontainers.image.source="https://github.com/neverlless/taskshed" \
      org.opencontainers.image.url="https://github.com/neverlless/taskshed" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}"

COPY --from=builder /out/taskshed /taskshed
COPY --from=builder --chown=65532:65532 /out/data /data

ENV DB_PATH=/data/taskshed.db
VOLUME /data
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/taskshed", "-healthcheck"]

USER nonroot:nonroot
ENTRYPOINT ["/taskshed"]

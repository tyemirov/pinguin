# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS="${TARGETOS:-$(go env GOOS)}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -o /workspace/bin/pingwin ./cmd/server && \
    GOOS="${TARGETOS:-$(go env GOOS)}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -o /workspace/bin/pingwin-doctor ./cmd/doctor && \
    GOOS="${TARGETOS:-$(go env GOOS)}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -o /workspace/bin/pingwin-convert-managed-tenants ./cmd/convert-managed-tenants

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /workspace/bin/pingwin /usr/local/bin/pingwin
COPY --from=builder /workspace/bin/pingwin-doctor /usr/local/bin/pingwin-doctor
COPY --from=builder /workspace/bin/pingwin-convert-managed-tenants /usr/local/bin/pingwin-convert-managed-tenants

VOLUME ["/web"]

EXPOSE 50051

CMD ["/usr/local/bin/pingwin"]

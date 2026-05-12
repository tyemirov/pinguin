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
    go build -o /workspace/bin/pinguin ./cmd/server && \
    GOOS="${TARGETOS:-$(go env GOOS)}" GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -o /workspace/bin/pinguin-doctor ./cmd/doctor

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /workspace/bin/pinguin /usr/local/bin/pinguin
COPY --from=builder /workspace/bin/pinguin-doctor /usr/local/bin/pinguin-doctor

VOLUME ["/web"]

EXPOSE 50051

CMD ["/usr/local/bin/pinguin"]

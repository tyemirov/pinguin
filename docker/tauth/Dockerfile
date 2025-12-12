FROM golang:1.25 AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends git ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

ARG TAUTH_REPO=https://github.com/tyemirov/TAuth.git
ARG TAUTH_REF=ba23dd61cb855a13f5c097701eae7f70fb4f72fb

RUN git clone --no-checkout ${TAUTH_REPO} . && \
    git checkout ${TAUTH_REF}

# Keep issuer compatible with Pinguin's session validator default (`tauth`).
RUN sed -i 's/AppJWTIssuer:      \"mprlab-auth\"/AppJWTIssuer:      \"tauth\"/' cmd/server/main.go

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/tauth ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    mkdir -p /data

COPY --from=builder /out/tauth /usr/local/bin/tauth

VOLUME ["/data"]
EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/tauth"]

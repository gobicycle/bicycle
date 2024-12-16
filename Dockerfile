FROM docker.io/library/golang:1.22.1-bullseye AS builder
WORKDIR /build-dir
COPY go.mod .
COPY go.sum .
RUN go mod download all
COPY api api
COPY blockchain blockchain
COPY cmd cmd
COPY config config
COPY core core
COPY db db
COPY audit audit
COPY queue queue
COPY webhook webhook
COPY metrics metrics
RUN apt-get update && apt-get install -y libsecp256k1-0 libsodium23
ARG GIT_TAG
RUN go build -ldflags "-X main.Version=$GIT_TAG" -o /tmp/processor github.com/gobicycle/bicycle/cmd/processor
RUN go build -ldflags "-X main.Version=$GIT_TAG" -o /tmp/testutil github.com/gobicycle/bicycle/cmd/testutil

FROM docker.io/library/ubuntu:20.04 AS payment-processor
RUN apt-get update && apt-get install -y openssl ca-certificates libsecp256k1-0 libsodium23 wget && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/lib
RUN wget -O /app/lib/libemulator.so https://github.com/ton-blockchain/ton/releases/download/v2024.09/libemulator-linux-x86_64.so
ENV LD_LIBRARY_PATH=/app/lib
COPY --from=builder /tmp/processor /app/processor
CMD ["/app/processor", "-v"]

FROM docker.io/library/ubuntu:20.04 AS payment-test
RUN apt-get update && apt-get install -y openssl ca-certificates libsecp256k1-0 libsodium23 wget && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/lib
RUN wget -O /app/lib/libemulator.so https://github.com/ton-blockchain/ton/releases/download/v2024.09/libemulator-linux-x86_64.so
ENV LD_LIBRARY_PATH=/app/lib
COPY --from=builder /tmp/testutil /app/testutil
CMD ["/app/testutil", "-v"]

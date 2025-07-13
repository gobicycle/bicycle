FROM docker.io/library/golang:1.23-bookworm AS builder
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
RUN apt-get update && apt-get install -y libsodium23
ARG GIT_TAG
RUN go build -ldflags "-X main.Version=$GIT_TAG" -o /tmp/processor github.com/gobicycle/bicycle/cmd/processor
RUN go build -ldflags "-X main.Version=$GIT_TAG" -o /tmp/testutil github.com/gobicycle/bicycle/cmd/testutil

FROM docker.io/library/ubuntu:24.04 AS payment-processor
RUN apt-get update && apt-get install -y openssl ca-certificates libsodium23 wget && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/lib
COPY --from=builder /go/pkg/mod/github.com/tonkeeper/tongo*/lib/linux /app/lib/
ENV LD_LIBRARY_PATH=/app/lib
COPY --from=builder /tmp/processor /app/processor
CMD ["/app/processor", "-v"]

FROM docker.io/library/ubuntu:24.04 AS payment-test
RUN apt-get update && apt-get install -y openssl ca-certificates libsodium23 wget && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/lib
COPY --from=builder /go/pkg/mod/github.com/tonkeeper/tongo*/lib/linux /app/lib/
ENV LD_LIBRARY_PATH=/app/lib
COPY --from=builder /tmp/testutil /app/testutil
CMD ["/app/testutil", "-v"]

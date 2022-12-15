FROM golang:1.19.2 AS builder
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
RUN go build -o /tmp/processor github.com/gobicycle/bicycle/cmd/processor
RUN go build -o /tmp/testutil github.com/gobicycle/bicycle/cmd/testutil
RUN git clone https://github.com/startfellows/tongo /tmp/tongo

FROM ubuntu:20.04 AS payment-processor
RUN apt-get update
RUN apt-get -y install zlib1g-dev libssl-dev
RUN mkdir -p /lib
COPY --from=builder /tmp/processor /app/processor
COPY --from=builder /tmp/tongo/lib/linux/libvm-exec-lib.so /lib
ENV LD_LIBRARY_PATH=/lib
CMD ["/app/processor", "-v"]

FROM ubuntu:20.04 AS payment-test
RUN apt-get update
RUN apt-get -y install zlib1g-dev libssl-dev
RUN mkdir -p /lib
COPY --from=builder /tmp/testutil /app/testutil
COPY --from=builder /tmp/tongo/lib/linux/libvm-exec-lib.so /lib
ENV LD_LIBRARY_PATH=/lib
CMD ["/app/testutil", "-v"]
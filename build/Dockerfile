FROM docker.io/library/ubuntu:20.04 AS emulator-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update
RUN apt-get -y install git cmake g++ zlib1g-dev libssl-dev
RUN git clone --recurse-submodules -b emulator_vm_verbosity https://github.com/dungeon-master-666/ton.git
RUN mkdir build && (cd build && cmake ../ton -DCMAKE_BUILD_TYPE=Release && cmake --build . --target emulator)
RUN mkdir /output && cp build/emulator/libemulator.so /output

FROM docker.io/library/golang:1.19.2 AS builder
WORKDIR /build-dir
COPY ../go.mod .
COPY ../go.sum .
RUN go mod download all
COPY ../api api
COPY ../internal/blockchain blockchain
COPY ../cmd cmd
COPY ../internal/config config
COPY ../internal/core core
COPY ../internal/database db
COPY ../internal/audit audit
COPY ../internal/queue queue
COPY ../internal/webhook webhook
RUN go build -o /tmp/processor github.com/gobicycle/bicycle/cmd/processor
RUN go build -o /tmp/testutil github.com/gobicycle/bicycle/cmd/testutil

FROM docker.io/library/ubuntu:20.04 AS payment-processor
RUN apt-get update && apt-get -y install zlib1g-dev libssl-dev openssl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /lib
COPY --from=builder /tmp/processor /app/processor
COPY --from=emulator-builder /output/libemulator.so /lib
ENV LD_LIBRARY_PATH=/lib
CMD ["/app/processor", "-v"]

FROM docker.io/library/ubuntu:20.04 AS payment-test
RUN apt-get update && apt-get -y install zlib1g-dev libssl-dev openssl ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /lib
COPY --from=builder /tmp/testutil /app/testutil
COPY --from=emulator-builder /output/libemulator.so /lib
ENV LD_LIBRARY_PATH=/lib
CMD ["/app/testutil", "-v"]
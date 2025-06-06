FROM golang:1.23 AS builder
WORKDIR /src
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o /bin/app ./cmd/snapshotter

FROM debian:stable-slim
RUN apt-get update && apt-get -y upgrade && apt-get install -y --no-install-recommends \
  libssl-dev \
  ca-certificates \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/app /snapshotter
ENTRYPOINT ["/snapshotter"]

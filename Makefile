.PHONY: lint test build

lint:
	docker run -t --rm -v $$(pwd):/app:ro -w /app golangci/golangci-lint:v2.1.2 golangci-lint run -v

test:
	go test -v ./...

build:
	go build -o snapshotter ./cmd/snapshotter

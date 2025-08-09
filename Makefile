.PHONY: build test run fmt

build:
\tgo build -o bin/edge-gateway ./cmd/gateway

fmt:
\tgofmt -w .

test:
\tgo test ./...

run:
\t./bin/edge-gateway --config config/config.yaml

.PHONY: dev build test clean

BINARY := bin/server

dev:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		go run ./cmd/server; \
	fi

build:
	go build -o $(BINARY) ./cmd/server

test:
	go test ./... -v

clean:
	rm -rf bin/

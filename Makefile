.PHONY: dev build test clean frontend build-all dev-frontend dev-backend

BINARY := bin/server

frontend:
	cd web && npm run build

build-all: frontend
	go build -o $(BINARY) ./cmd/server

build:
	go build -o $(BINARY) ./cmd/server

dev-frontend:
	cd web && npm run dev

dev-backend:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		go run ./cmd/server; \
	fi

dev:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		go run ./cmd/server; \
	fi

test:
	go test ./... -v

clean:
	rm -rf bin/

.PHONY: dev build test clean frontend build-all dev-frontend dev-backend

BINARY := build/zhipu

frontend:
	cd frontend && npm run build

build-all: frontend
	go build -o $(BINARY) .

build:
	go build -o $(BINARY) .

dev-frontend:
	cd frontend && npm run dev

dev-backend:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		go run .; \
	fi

dev:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		go run .; \
	fi

test:
	go test ./... -v

clean:
	rm -rf build/

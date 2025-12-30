GO ?= go
PROTOC ?= protoc
CONFIG ?= .
WAILS ?= wails3
VERSION ?= dev
BUILD ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X mcpd/internal/app.Version=$(VERSION) -X mcpd/internal/app.Build=$(BUILD)

.PHONY: dev proto

build:
	$(GO) build -ldflags "$(LDFLAGS)" ./...

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

proto:
	$(PROTOC) -I proto \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=module=mcpd \
		--go-grpc_opt=module=mcpd \
		proto/mcpd/control/v1/control.proto

# Docker Compose development environment
dev:
	docker compose up -d

down:
	docker compose down

reload:
	docker compose restart core

# Wails application targets
wails-bindings:
	$(WAILS) generate bindings -ts ./cmd/mcpd-wails

wails-dev:
	$(WAILS) dev

wails-build:
	$(WAILS) build

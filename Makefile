GO ?= go
PROTOC ?= protoc
CONFIG ?= .
WAILS ?= wails3
VERSION ?= dev
BUILD ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X mcpd/internal/app.Version=$(VERSION) -X mcpd/internal/app.Build=$(BUILD)
BIN_DIR ?= $(CURDIR)/bin
WIRE := $(BIN_DIR)/wire

.PHONY: dev obs down reload proto wire tools

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

tools: $(WIRE)

$(WIRE):
	GOBIN=$(BIN_DIR) $(GO) install github.com/google/wire/cmd/wire@latest

wire:
	$(WIRE) ./internal/app

# Docker Compose development environment
dev:
	docker compose up -d

obs:
	MCPD_PROM_CONFIG=./dev/prometheus.wails.yaml docker compose up -d prometheus grafana

down:
	docker compose down

reload:
	docker compose restart core

# Wails application targets
wails-bindings:
	$(WAILS) generate bindings -ts

wails-dev:
	$(WAILS) dev

wails-build:
	$(WAILS) build

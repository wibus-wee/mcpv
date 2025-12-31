GO ?= go
PROTOC ?= protoc
CONFIG ?= .
VERSION ?= dev
BUILD ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X mcpd/internal/app.Version=$(VERSION) -X mcpd/internal/app.Build=$(BUILD)
TOOLCHAIN ?= go1.25.0
BIN_DIR ?= $(CURDIR)/bin
WIRE := $(BIN_DIR)/wire
WAILS ?= $(BIN_DIR)/wails3

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

tools: $(WIRE) $(WAILS)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(WIRE): | $(BIN_DIR)
	GOTOOLCHAIN=$(TOOLCHAIN) GOBIN=$(BIN_DIR) $(GO) install github.com/google/wire/cmd/wire@latest

$(BIN_DIR)/wails: | $(BIN_DIR)
	GOTOOLCHAIN=$(TOOLCHAIN) GOBIN=$(BIN_DIR) $(GO) install github.com/wailsapp/wails/v3/cmd/wails3@latest

wire: $(WIRE)
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

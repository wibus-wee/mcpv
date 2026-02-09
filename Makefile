MACOS_MIN ?= 11.0
GO ?= go
PROTOC ?= protoc
CONFIG ?= .
WAILS ?= $(BIN_DIR)/wails3
VERSION ?= dev
BUILD ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X mcpv/internal/buildinfo.Version=$(VERSION) -X mcpv/internal/buildinfo.Build=$(BUILD)
BIN_DIR ?= $(CURDIR)/bin
WIRE := $(BIN_DIR)/wire

.PHONY: dev obs down reload proto wire tools build

export MACOSX_DEPLOYMENT_TARGET=$(MACOS_MIN)
export CGO_CFLAGS=-mmacosx-version-min=$(MACOS_MIN)
export CGO_CXXFLAGS=-mmacosx-version-min=$(MACOS_MIN)
export CGO_LDFLAGS=-mmacosx-version-min=$(MACOS_MIN)

install-toolchain:
	$(MAKE) $(WIRE) $(WAILS)

build:
	mkdir -p bin/core
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/core/mcpv ./cmd/mcpv
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/core/mcpvmcp ./cmd/mcpvmcp

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint-check:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install it with: brew install golangci-lint"; \
		exit 1; \
	}
	golangci-lint run --config .golangci.yml

lint-fix:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install it with: brew install golangci-lint"; \
		exit 1; \
	}
	golangci-lint run --config .golangci.yml --fix

proto:
	$(PROTOC) -I proto \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=module=mcpv \
		--go-grpc_opt=module=mcpv \
		proto/mcpv/control/v1/control.proto \
		proto/mcpv/plugin/v1/plugin.proto

$(WIRE):
	GOBIN=$(BIN_DIR) $(GO) install github.com/google/wire/cmd/wire@latest

$(WAILS):
	GOBIN=$(BIN_DIR) $(GO) install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.70

wire:
	$(WIRE) ./internal/app

# Docker Compose development environment
dev:
	docker compose up -d

obs:
	mcpv_PROM_CONFIG=./dev/prometheus.wails.yaml docker compose up -d prometheus grafana

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

wails-package:
	$(WAILS) package

GO ?= go
PROTOC ?= protoc
CONFIG ?= docs/catalog.example.yaml
WAILS ?= wails3

.PHONY: dev proto

build:
	$(GO) build ./...

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

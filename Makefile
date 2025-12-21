GO ?= go
CONFIG ?= catalog.yaml

.PHONY: build test fmt tidy vet serve validate

build:
	$(GO) build ./...

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

vet:
	$(GO) vet ./...

serve:
	$(GO) run ./cmd/mcpd serve --config $(CONFIG)

validate:
	$(GO) run ./cmd/mcpd validate --config $(CONFIG)

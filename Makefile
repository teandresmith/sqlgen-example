# Task-tracker example — developer workflow targets.
#
# The sqlgen CLI is installed from github.com/teandresmith/sqlgen at the same
# version go.mod pins for the runtime, so the generator and the runtime the
# generated code compiles against never drift apart. Migrations are the single
# source of truth (ADR-0002) and feed both golang-migrate and sqlgen's parser.

# Read from go.mod; bump the sqlgen requirement there (go get
# github.com/teandresmith/sqlgen@vX.Y.Z) and the CLI follows. The CLI is its own
# nested module (cmd/sqlgen), tagged in lockstep with the runtime.
SQLGEN_VERSION ?= $(shell go list -m -f '{{.Version}}' github.com/teandresmith/sqlgen)
SQLGEN_BIN ?= ./bin/sqlgen

# The generated manifest — the machine-readable description of the client that
# `manifest-validate` checks and the MCP server (and .mcp.json) serve.
MANIFEST ?= ./internal/database/manifest/manifest_gen.json

# Local connection strings — match docker-compose.yml. Override on the command
# line for other environments, e.g. `make migrate DATABASE_URL=...`.
export DATABASE_URL ?= postgres://taskr:taskr@localhost:5432/taskr?sslmode=disable
export REDIS_URL    ?= redis://localhost:6379
export NATS_URL     ?= nats://localhost:4222
export JWT_SECRET   ?= dev-secret-change-me
export OTEL_EXPORTER_OTLP_ENDPOINT ?= localhost:4317

.PHONY: help up down sqlgen generate diff manifest-validate mcp verify migrate run seed test test-short build vet tidy

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

up: ## Start Postgres, Redis, NATS, and the OTel collector
	docker compose up -d

down: ## Stop the local stack
	docker compose down

sqlgen: ## Install the sqlgen CLI (the version go.mod pins) into ./bin
	GOBIN=$(abspath $(dir $(SQLGEN_BIN))) go install github.com/teandresmith/sqlgen/cmd/sqlgen@$(SQLGEN_VERSION)

generate: sqlgen ## Regenerate the client + GraphQL surface from ./migrations
	$(SQLGEN_BIN) generate

diff: sqlgen ## Fail if the generated client is stale vs ./migrations (CI drift gate)
	$(SQLGEN_BIN) diff

manifest-validate: sqlgen ## Validate the generated manifest against its JSON schema
	$(SQLGEN_BIN) manifest validate $(MANIFEST)

mcp: sqlgen ## Serve the generated data-model surface to AI agents over MCP (stdio)
	$(SQLGEN_BIN) mcp serve --manifest $(MANIFEST)

verify: build vet diff manifest-validate ## Fast local gate (no Docker): build, vet, drift, manifest

migrate: ## Apply ./migrations to DATABASE_URL
	go run ./cmd/migrate

run: ## Run the API server (applies migrations at startup)
	go run ./cmd/server

seed: ## Populate a realistic multi-workspace dataset (applies migrations first)
	go run ./cmd/seed

build: ## Compile all packages
	go build ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy the module graph
	go mod tidy

test: ## Run the full test suite (needs Docker for testcontainers)
	go test ./...

test-short: ## Run unit tests only (skips container-backed integration tests)
	go test -short ./...

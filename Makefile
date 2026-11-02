-include .env
export

BINARY := bin/api

TRUTH ?= truth.ndjson

.PHONY: help env run run-worker evaluate build test fmt vet tidy up infra down logs ps clean

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-11s %s\n", $$1, $$2}'

env: ## Create .env from .env.example
	@test -f .env || cp .env.example .env

run: ## Run the API locally
	go run ./cmd/api

run-worker: ## Run the analysis worker locally
	go run ./cmd/worker

evaluate: ## Score decisions against TRUTH (default truth.ndjson)
	go run ./cmd/evaluator --truth=$(TRUTH)

build: ## Build the API binary
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/api

test: ## Run tests with race detector
	go test -race ./...

fmt: ## Format code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod
	go mod tidy

up: env ## Start the full stack (api + worker + infra)
	docker compose up -d --build

infra: env ## Start only infrastructure (postgres, redis, kafka)
	docker compose up -d postgres redis kafka

down: ## Stop the stack
	docker compose down

logs: ## Follow logs
	docker compose logs -f

ps: ## Show containers status
	docker compose ps

clean: ## Remove build output and volumes
	rm -rf bin
	docker compose down -v

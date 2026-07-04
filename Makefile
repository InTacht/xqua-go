GO         ?= go
PKGS       ?= ./...
COVERFILE  ?= coverage.out

# Tag used by tests that require external services (e.g. the dev Postgres).
INTEGRATION_TAG ?= integration

# Dev environment: a Postgres started via docker compose, configured from .env.dev.
COMPOSE_FILE ?= docker-compose.dev.yaml
ENV_FILE     ?= .env.dev

# Resolve the docker command lazily, adding sudo only when the daemon is not
# reachable directly. Evaluated only when a compose target actually uses it.
DOCKER = $(shell if docker info >/dev/null 2>&1; then echo docker; else echo "sudo docker"; fi)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Compile all packages
	$(GO) build $(PKGS)

.PHONY: test
test: ## Run unit tests (no external services)
	$(GO) test $(PKGS)

.PHONY: test-race
test-race: ## Run unit tests with the race detector
	$(GO) test -race $(PKGS)

.PHONY: dev-up
dev-up: ## Start the dev Postgres (docker compose) and wait until healthy
	$(DOCKER) compose -f $(COMPOSE_FILE) up -d --wait

.PHONY: dev-down
dev-down: ## Stop the dev Postgres
	$(DOCKER) compose -f $(COMPOSE_FILE) down

.PHONY: dev-reset
dev-reset: ## Stop the dev Postgres and delete its data volume
	$(DOCKER) compose -f $(COMPOSE_FILE) down -v

.PHONY: test-integration
test-integration: dev-up ## Start dev DB if needed, then run integration tests
	@set -a; . ./$(ENV_FILE); set +a; \
		$(GO) test -tags=$(INTEGRATION_TAG) -count=1 $(PKGS)

.PHONY: test-all
test-all: test test-integration ## Run unit and integration tests

.PHONY: cover
cover: ## Run unit tests with coverage and print a summary
	$(GO) test -coverprofile=$(COVERFILE) $(PKGS)
	$(GO) tool cover -func=$(COVERFILE) | tail -1

.PHONY: cover-html
cover-html: cover ## Generate an HTML coverage report at coverage.html
	$(GO) tool cover -html=$(COVERFILE) -o coverage.html

.PHONY: vet
vet: ## Run go vet, including integration-tagged files
	$(GO) vet $(PKGS)
	$(GO) vet -tags=$(INTEGRATION_TAG) $(PKGS)

.PHONY: fmt
fmt: ## Format all Go source
	$(GO) fmt $(PKGS)

.PHONY: fmt-check
fmt-check: ## Fail if any Go source is not gofmt-clean
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "gofmt needed on:"; echo "$$files"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Sync go.mod/go.sum
	$(GO) mod tidy

.PHONY: check
check: fmt-check vet test ## Format check, vet, and unit tests (pre-commit gate)

.PHONY: clean
clean: ## Remove build/test artifacts and caches
	$(GO) clean -testcache
	rm -f $(COVERFILE) coverage.html

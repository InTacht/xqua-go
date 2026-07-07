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

# Release workflow: update CHANGELOG.md, commit, then:
#   make release VERSION=v0.2.0
# Optional: NOTES=path/to/notes.txt to override changelog extraction.
VERSION ?=
NOTES   ?=

# Strip the leading v from VERSION for CHANGELOG section headers (e.g. v0.1.0 -> 0.1.0).
CHANGELOG_VERSION = $(patsubst v%,%,$(VERSION))

define changelog_notes
awk -v ver="$(CHANGELOG_VERSION)" ' \
	$$0 ~ "^## \\[" ver "\\] " { found=1; next } \
	found && /^## \[/ { exit } \
	found { print } \
' CHANGELOG.md
endef

.PHONY: release
release: release-check release-tag release-push release-github ## Tag, push, and publish a GitHub release (VERSION=vX.Y.Z)

.PHONY: release-check
release-check: check ## Verify VERSION, clean tree, and changelog section
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=v0.2.0"; exit 1; }
	@case "$(VERSION)" in v[0-9]*.[0-9]*.[0-9]*) ;; *) \
		echo "VERSION must be semver with a v prefix (e.g. v0.2.0)"; exit 1;; esac
	@test -z "$$(git status --porcelain)" || { echo "working tree is not clean"; exit 1; }
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "tag $(VERSION) already exists"; exit 1; \
	fi
	@if [ -z "$(NOTES)" ]; then \
		$(changelog_notes) | grep -q . || { echo "no CHANGELOG section for $(VERSION)"; exit 1; }; \
	fi

.PHONY: release-tag
release-tag: ## Create an annotated git tag from CHANGELOG or NOTES
	@notes=$$( \
		if [ -n "$(NOTES)" ]; then cat "$(NOTES)"; \
		else $(changelog_notes); \
		fi \
	); \
	git tag -a "$(VERSION)" -m "$$notes"
	@echo "created tag $(VERSION)"

.PHONY: release-push
release-push: ## Push the release tag to origin
	git push origin "$(VERSION)"
	@echo "pushed $(VERSION) to origin"

.PHONY: release-github
release-github: ## Create a GitHub release when gh is available
	@if command -v gh >/dev/null 2>&1; then \
		notes=$$(mktemp); \
		if [ -n "$(NOTES)" ]; then cat "$(NOTES)" > "$$notes"; \
		else $(changelog_notes) > "$$notes"; \
		fi; \
		gh release create "$(VERSION)" --title "$(VERSION)" --notes-file "$$notes"; \
		rm -f "$$notes"; \
	else \
		echo "gh not found; skipped GitHub release (push tag manually or install gh)"; \
	fi

.PHONY: clean
clean: ## Remove build/test artifacts and caches
	$(GO) clean -testcache
	rm -f $(COVERFILE) coverage.html

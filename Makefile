# =============================================================================
# FMCG Wallet — Makefile
# Common development commands. Run `make help` to list all targets.
# =============================================================================

# Detect OS for cross-platform commands
ifeq '$(findstring ;,$(PATH))' ';'
	IS_WINDOWS := 1
	RM_RF := rmdir /s /q 2>nul & exit 0
	MKDIR_P := mkdir
	SHELL_CMD := cmd /c
else
	IS_WINDOWS :=
	RM_RF := rm -rf
	MKDIR_P := mkdir -p
	SHELL_CMD :=
endif

BIN_DIR := bin
COVERAGE_FILE := coverage.out

# Default target
.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================
.PHONY: help
help: ## Show this help (default)
	@echo "FMCG Wallet — available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# Setup & Bootstrap
# =============================================================================
.PHONY: setup
setup: ## First-time setup: install git hooks, verify tools
	@echo "Verifying required tools..."
	@go version >/dev/null 2>&1 || (echo "ERROR: go not installed" && exit 1)
	@echo "Go: $$(go version)"
	@git --version
	@echo "Setup OK."

# =============================================================================
# Docker Compose (local dev stack)
# =============================================================================
.PHONY: up
up: ## Start local docker-compose stack in background
	docker compose up -d

.PHONY: down
down: ## Stop local docker-compose stack
	docker compose down

.PHONY: down-volumes
down-volumes: ## Stop stack AND delete volumes (DESTRUCTIVE)
	docker compose down -v

.PHONY: logs
logs: ## Tail all docker compose logs
	docker compose logs -f

.PHONY: ps
ps: ## List running containers
	docker compose ps

.PHONY: restart
restart: down up ## Restart local stack

# =============================================================================
# Database
# =============================================================================
.PHONY: migrate-up
migrate-up: ## Run all up migrations
	go run ./cmd/migrator up

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	go run ./cmd/migrator down 1

.PHONY: migrate-status
migrate-status: ## Show migration status
	go run ./cmd/migrator status

.PHONY: db-shell
db-shell: ## Open psql in the dev postgres container
	docker compose exec postgres psql -U fmcg -d fmcg_wallet

.PHONY: db-redis-cli
db-redis-cli: ## Open redis-cli in the dev redis container
	docker compose exec redis redis-cli

.PHONY: seed-local
seed-local: ## Seed demo data into local Postgres (idempotent)
	bash scripts/seed-local-data.sh

.PHONY: bcrypt-hash
bcrypt-hash: ## Generate a bcrypt hash for a demo password
	@go run scripts/gen-bcrypt-hash.go $(word 2,$(MAKECMDGOALS)) $(word 3,$(MAKECMDGOALS))
	@: # no-op so make doesn't complain about extra args

# =============================================================================
# Code generation
# =============================================================================
.PHONY: sqlc-gen
sqlc-gen: ## Generate sqlc code from queries
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate; \
	else \
		echo "sqlc not installed. Install: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"; \
		exit 1; \
	fi

.PHONY: swag-gen
swag-gen: ## Generate Swagger/OpenAPI docs
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/api/main.go -o docs/api/swagger; \
	else \
		echo "swag not installed. Install: go install github.com/swaggo/swag/cmd/swag@latest"; \
		exit 1; \
	fi

# =============================================================================
# Quality
# =============================================================================
.PHONY: fmt
fmt: ## Format Go code (gofmt + goimports)
	gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "(goimports not installed, skipping import sort)"; \
	fi

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (strict)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m ./...; \
	else \
		echo "golangci-lint not installed. Install: https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	fi

.PHONY: test
test: ## Run all tests with race detector
	go test -race -shuffle=on ./...

.PHONY: test-cover
test-cover: ## Run tests + open coverage report
	go test -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -func=$(COVERAGE_FILE) | tail -1
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-cover-check
test-cover-check: ## Run tests + enforce minimum coverage threshold (80%)
	@bash -c 'set -e; \
		go test -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./... > /tmp/test.log 2>&1; \
		COVERAGE=$$(go tool cover -func=$(COVERAGE_FILE) | grep total | awk "{print \$3}" | sed "s/%//"); \
		echo "Total coverage: $${COVERAGE}%"; \
		THRESHOLD=80; \
		awk -v c=$$COVERAGE -v t=$$THRESHOLD "BEGIN {if (c+0 < t+0) {print \"ERROR: coverage \" c \"% below threshold \" t \"%\"; exit 1} else {print \"OK: coverage \" c \"% >= \" t \"%\"}}"; \
	'

.PHONY: security
security: ## Run security scanners
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo "gosec not installed. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed. Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

.PHONY: mutate
mutate: ## Run mutation testing (slow)
	@if command -v go-mutesting >/dev/null 2>&1; then \
		go-mutesting ./...; \
	else \
		echo "go-mutesting not installed. Install: go install github.com/avito-tech/go-mutesting/...@latest"; \
	fi

# =============================================================================
# Build
# =============================================================================
.PHONY: build
build: ## Build all binaries into ./bin
	$(MKDIR_P) $(BIN_DIR)
	go build -trimpath -ldflags="-s -w -X main.version=$$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o $(BIN_DIR)/api ./cmd/api
	go build -trimpath -ldflags="-s -w -X main.version=$$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o $(BIN_DIR)/worker ./cmd/worker
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/migrator ./cmd/migrator

.PHONY: build-api
build-api: ## Build only API binary
	$(MKDIR_P) $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/api ./cmd/api

.PHONY: docker-build
docker-build: ## Build Docker image (requires Docker)
	docker build -t fmcg-wallet:$$(git rev-parse --short HEAD 2>/dev/null || echo dev) .

# =============================================================================
# Run
# =============================================================================
.PHONY: run-api
run-api: ## Run API server (requires DB up)
	go run ./cmd/api

.PHONY: run-worker
run-worker: ## Run background worker
	go run ./cmd/worker

# =============================================================================
# Housekeeping
# =============================================================================
.PHONY: clean
clean: ## Clean build artifacts
	$(RM_RF) $(BIN_DIR) 2>nul
	$(RM_RF) $(COVERAGE_FILE) coverage.html 2>nul

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: verify
verify: fmt vet lint test-cover-check ## Full verification (fmt + vet + lint + test + coverage)
	@echo ""
	@echo "=========================================="
	@echo "  ALL CHECKS PASSED"
	@echo "=========================================="

.PHONY: help build run test clean \
		dev-up dev-down dev-logs dev-restart \
		dev-docker dev-docker-down dev-docker-logs \
		dev-local dev-local-down \
		db-reset db-migrate db-migrate-create db-shell db-dump \
		test-all test-unit test-integration lint fmt check

# Docker compose configuration
DC_ROOT := docker compose -f ../docker-compose.yml
DC_DEV := docker compose -f ../docker-compose.dev.yml

# Database configuration (matches docker-compose.dev.yml)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= wikikeeper
DB_PASSWORD ?= wikikeeper123
DB_NAME ?= wikikeeper

# Go configuration
GOCMD := go
GOTEST := $(GOCMD) test
GOBUILD := $(GOCMD) build
GOMOD := $(GOCMD) mod

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo '🚀 Quick Start:'
	@echo '  dev-docker           Start ALL services in Docker (postgres + backend)'
	@echo '  dev-local            Start DB in Docker, run backend locally'
	@echo ''
	@echo '🐳 Docker Development (ALL in Docker):'
	@echo '  dev-docker           Start all services in Docker'
	@echo '  dev-docker-down      Stop all Docker services'
	@echo '  dev-docker-logs      Show logs from all Docker services'
	@echo '  dev-docker-tools     Start with Adminer (DB UI at :8080)'
	@echo ''
	@echo '💻 Local Development (DB only in Docker):'
	@echo '  dev-local            Start DB in Docker'
	@echo '  dev-local-down       Stop Docker DB'
	@echo '  run                  Run backend locally'
	@echo '  run-dev              Run backend with hot reload'
	@echo '  frontend-dev         Start frontend locally'
	@echo ''
	@echo '🗄️  Database Operations:'
	@echo '  db-migrate           Run database migrations'
	@echo '  db-reset             Reset database (drop, create, migrate)'
	@echo '  db-shell             Open PostgreSQL shell'
	@echo '  db-dump FILE=f.sql   Dump database to file'
	@echo ''
	@echo '🔧 Build & Test:'
	@echo '  build                Build backend'
	@echo '  deps                 Install Go dependencies'
	@echo '  test                 Run tests (short mode)'
	@echo '  test-all             Run all tests'
	@echo '  test-coverage        Generate coverage report'
	@echo ''
	@echo '✨ Code Quality:'
	@echo '  fmt                  Format code'
	@echo '  lint                 Run linter'
	@echo '  staticcheck          Run staticcheck'
	@echo '  check                Run all checks'
	@echo ''
	@echo '🧹 Cleanup:'
	@echo '  clean                Clean build artifacts'
	@echo '  clean-all            Clean everything including volumes'
	@echo ''
	@echo '🛠️  Tools:'
	@echo '  install-tools        Install development tools'

# =============================================================================
# 🚀 Quick Start Commands
# =============================================================================

dev-docker: ## Start all services in Docker (recommended)
	@echo "Starting all services in Docker..."
	@echo ""
	$(DC_DEV) up -d
	@echo "Waiting for services..."
	@sleep 5
	@until docker exec wikikeeper-postgres-dev pg_isready -U wikikeeper > /dev/null 2>&1; do sleep 1; done
	@echo ""
	@echo "All services started!"
	@echo ""
	@echo "Services:"
	@echo "  - App:       http://localhost:8000"
	@echo "  - Database:  localhost:5432"

dev-local: ## Start DB in Docker, run backend locally
	@echo "Starting development environment (local mode)..."
	@echo ""
	@make dev-local-db
	@echo ""
	@echo "Database is ready!"
	@echo ""
	@echo "Now run:"
	@echo "  make run-dev      (backend with hot reload)"
	@echo ""
	@echo "Or use docker for everything: make dev-docker"

# =============================================================================
# 🐳 Docker Development (ALL in Docker)
# =============================================================================

dev-up: dev-docker ## Alias for dev-docker

dev-docker: ## Start all services in Docker
	@echo "Starting all services in Docker..."
	$(DC_DEV) up -d

dev-down: dev-docker-down ## Alias for dev-docker-down

dev-docker-down: ## Stop all Docker services
	$(DC_DEV) down
	@echo "Docker services stopped"

dev-logs: dev-docker-logs ## Alias for dev-docker-logs

dev-docker-logs: ## Show logs from all Docker services
	$(DC_DEV) logs -f

dev-docker-tools: ## Start all services + Adminer (DB UI at :8080)
	@echo "Starting all services with Adminer..."
	$(DC_DEV) --profile tools up -d
	@echo "Services started with Adminer at http://localhost:8080"

dev-restart: ## Restart all Docker services
	$(DC_DEV) restart
	@echo "Services restarted"

# =============================================================================
# 💻 Local Development (DB only in Docker)
# =============================================================================

dev-local-db: ## Start database only in Docker
	@echo "Starting PostgreSQL in Docker..."
	$(DC_DEV) up -d postgres
	@echo "Waiting for database..."
	@until docker exec wikikeeper-postgres-dev pg_isready -U wikikeeper > /dev/null 2>&1; do sleep 1; done
	@echo ""
	@echo "PostgreSQL is ready!"

dev-local-down: ## Stop Docker database
	$(DC_DEV) down
	@echo "✅ Database stopped"

dev-db: dev-local-db ## Alias for dev-local-db

dev-db-logs: ## Show database logs
	$(DC_DEV) logs -f postgres

# =============================================================================
# 🗄️ Database Operations
# =============================================================================

db-reset: ## Reset database (drop, create, migrate)
	@echo "🔄 Resetting database..."
	docker exec wikikeeper-postgres-dev psql -U wikikeeper -c "DROP DATABASE IF EXISTS wikikeeper;"
	docker exec wikikeeper-postgres-dev psql -U wikikeeper -c "CREATE DATABASE wikikeeper;"
	$(MAKE) db-migrate
	@echo "✅ Database reset complete!"

db-migrate: ## Run database migrations
	@echo "🔄 Running database migrations..."
	@for migration in migrations/*.up.sql; do \
		echo "Executing $$migration..."; \
		docker exec -i wikikeeper-postgres-dev psql -U wikikeeper -d wikikeeper < $$migration; \
	done
	@echo "✅ Migrations complete!"

db-migrate-create: ## Create a new migration (use NAME=migration_name)
	@if [ -z "$(NAME)" ]; then echo "❌ Usage: make db-migrate-create NAME=migration_name"; exit 1; fi
	@TIMESTAMP=$$(date +%Y%m%d%H%M%S); \
	echo "📝 Creating migration: $$TIMESTAMP_$(NAME)"; \
	touch migrations/$$TIMESTAMP_$(NAME).up.sql; \
	touch migrations/$$TIMESTAMP_$(NAME).down.sql; \
	echo "-- Migration: $(NAME)" > migrations/$$TIMESTAMP_$(NAME).up.sql; \
	echo "-- Migration: $(NAME)" > migrations/$$TIMESTAMP_$(NAME).down.sql; \
	echo "✅ Created files:"; \
	echo "  - migrations/$$TIMESTAMP_$(NAME).up.sql"; \
	echo "  - migrations/$$TIMESTAMP_$(NAME).down.sql"

db-shell: ## Open PostgreSQL shell
	docker exec -it wikikeeper-postgres-dev psql -U wikikeeper -d wikikeeper

db-dump: ## Dump database to file (use FILE=output.sql)
	docker exec wikikeeper-postgres-dev pg_dump -U wikikeeper wikikeeper > $(FILE:=dump.sql)
	@echo "✅ Database dumped to $(FILE:=dump.sql)"

# =============================================================================
# 🔧 Build & Run
# =============================================================================

deps: ## Install Go dependencies
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the application
	$(GOBUILD) -o bin/server cmd/server/main.go
	@echo "✅ Built bin/server"

run: ## Run the application locally (requires dev DB running)
	@echo "▶️  Starting server..."
	@echo "⚠️  Make sure dev database is running: make dev-local-db"
	$(GOCMD) run cmd/server/main.go

run-dev: ## Run with hot reload (requires air)
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	air

# =============================================================================
# 🧪 Testing
# =============================================================================

test: ## Run all tests (short mode)
	$(GOTEST) -v -short ./...

test-unit: ## Run unit tests (short mode)
	$(GOTEST) -v -short ./...

test-all: ## Run all tests including integration
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage report
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

# =============================================================================
# ✨ Code Quality
# =============================================================================

lint: ## Run linter
	@command -v golangci-lint >/dev/null 2>&1 || { echo "⚠️  golangci-lint not installed. Run: make install-tools"; exit 1; }
	golangci-lint run

fmt: ## Format code
	$(GOCMD) fmt ./...
	@echo "✅ Code formatted"

check: fmt lint test ## Run all checks (fmt, lint, test)

staticcheck: ## Run staticcheck
	@command -v staticcheck >/dev/null 2>&1 || { echo "⚠️  staticcheck not installed. Run: make install-tools"; exit 1; }
	staticcheck ./...

# =============================================================================
# 🧹 Cleanup
# =============================================================================

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -rf tmp/
	@echo "✅ Cleaned build artifacts"

clean-all: clean ## Clean everything including Docker volumes
	$(DC_DEV) down -v
	$(DC_ROOT) down -v 2>/dev/null || true
	@echo "✅ All cleaned up!"

# =============================================================================
# 🛠️ Tools
# =============================================================================

install-tools: ## Install all development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/air-verse/air@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "✅ Development tools installed!"
	@echo ""
	@echo "Tools installed:"
	@echo "  ✓ golangci-lint: Go linter"
	@echo "  ✓ staticcheck:   Static analysis"
	@echo "  ✓ air:          Live reload for Go"
	@echo "  ✓ migrate:      Database migration tool"

.DEFAULT_GOAL := help

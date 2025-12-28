.PHONY: help build run test clean migrate-up migrate-down deps

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o bin/server cmd/server/main.go

run: ## Run the application
	go run cmd/server/main.go

test: ## Run tests
	go test -v -race -cover ./...

test-coverage: ## Run tests with coverage report
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

migrate-up: ## Run database migrations up
	migrate -path migrations -database "postgres://wikitracker:wikitracker_password@localhost:5432/wikitracker?sslmode=disable" up

migrate-down: ## Run database migrations down
	migrate -path migrations -database "postgres://wikitracker:wikitracker_password@localhost:5432/wikitracker?sslmode=disable" down

migrate-create: ## Create a new migration (use NAME=name)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	migrate create -ext sql -dir migrations $(NAME)

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

lint: ## Run linter
	golangci-lint run

fmt: ## Format code
	go fmt ./...

install-migrate: ## Install golang-migrate tool
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.DEFAULT_GOAL := help

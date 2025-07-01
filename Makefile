.PHONY: build run test clean docker migrate

# Variables
DOCKER_COMPOSE = docker-compose -f deployments/docker/docker-compose.yml
DATABASE_URL ?= postgres://postgres:secretpassword@localhost:5432/domainmonitor?sslmode=disable

# Build commands
build:
	@echo "Building binaries..."
	@go build -o bin/api cmd/api/main.go
	@go build -o bin/worker cmd/worker/main.go
	@go build -o bin/scheduler cmd/scheduler/main.go

# Run commands
run-api:
	@echo "Running API server..."
	@go run cmd/api/main.go

run-worker:
	@echo "Running worker..."
	@go run cmd/worker/main.go

run-scheduler:
	@echo "Running scheduler..."
	@go run cmd/scheduler/main.go

# Docker commands
docker-up:
	@echo "Starting services with Docker Compose..."
	@$(DOCKER_COMPOSE) up -d

docker-down:
	@echo "Stopping services..."
	@$(DOCKER_COMPOSE) down

docker-logs:
	@$(DOCKER_COMPOSE) logs -f

docker-build:
	@echo "Building Docker images..."
	@$(DOCKER_COMPOSE) build

# Database commands
migrate-up:
	@echo "Running migrations..."
	@migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@echo "Rolling back migrations..."
	@migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@echo "Creating new migration..."
	@migrate create -ext sql -dir migrations -seq $(name)

# Development
dev:
	@echo "Starting development environment..."
	@$(DOCKER_COMPOSE) up -d postgres redis
	@sleep 3
	@make migrate-up
	@air

# Testing
test:
	@echo "Running tests..."
	@go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# Cleanup
clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -f coverage.out

# Installation
install-tools:
	@echo "Installing development tools..."
	@go install github.com/air-verse/air@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build all binaries"
	@echo "  run-api        - Run API server"
	@echo "  run-worker     - Run worker"
	@echo "  run-scheduler  - Run scheduler"
	@echo "  docker-up      - Start all services with Docker"
	@echo "  docker-down    - Stop all services"
	@echo "  docker-logs    - View Docker logs"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback last migration"
	@echo "  dev            - Start development environment"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
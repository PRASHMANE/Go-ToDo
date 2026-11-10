.PHONY: help tidy fmt vet

MODULES := pkg $(wildcard services/*)

help:            ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

tidy:            ## go mod tidy in every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && GOWORK=off go mod tidy); done

fmt:             ## gofmt everything
	gofmt -s -w .

vet:             ## go vet every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go vet ./...); done

TODO_DB_URL ?= postgres://todo_user:todo_pass@localhost:15432/todo_db?sslmode=disable

db-up:           ## Start Postgres
	docker compose up -d --wait postgres

migrate-todo:    ## Apply todo-service migrations
	migrate -path services/todo-service/migrations -database "$(TODO_DB_URL)" up

run-todo:        ## Run todo-service locally (Postgres)
	cd services/todo-service && DATABASE_URL="$(TODO_DB_URL)" NATS_URL="nats://localhost:4222" go run ./cmd

AUTH_DB_URL ?= postgres://auth_user:auth_pass@localhost:15432/auth_db?sslmode=disable
JWT_SECRET  ?= dev-only-secret-change-me-dev-only-secret

migrate-auth:    ## Apply auth-service migrations
	migrate -path services/auth-service/migrations -database "$(AUTH_DB_URL)" up

run-auth:        ## Run auth-service locally
	cd services/auth-service && DATABASE_URL="$(AUTH_DB_URL)" JWT_SECRET="$(JWT_SECRET)" go run ./cmd

run-gateway:     ## Run api-gateway locally
	cd services/api-gateway && JWT_SECRET="$(JWT_SECRET)" go run ./cmd

up:              ## Build and start everything
	docker compose up -d --build --wait

down:            ## Stop everything (keeps data)
	docker compose down

reset:           ## Stop everything and DELETE the database volume
	docker compose down -v

logs:            ## Follow logs (make logs s=todo-service)
	docker compose logs -f $(s)

ps:              ## Show container status
	docker compose ps

open:            ## Open the app
	open http://localhost:3000

test:            ## Unit + handler tests (race detector on)
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go test -race -cover ./...); done

test-integration: ## Integration tests (needs `make db-up migrate-todo`)
	cd services/todo-service && TEST_DATABASE_URL="$(TODO_DB_URL)" go test -tags=integration -race ./...

lint:            ## golangci-lint on every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && golangci-lint run ./...); done

cover:           ## HTML coverage report for todo-service
	cd services/todo-service && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

ACTIVITY_DB_URL ?= postgres://activity_user:activity_pass@localhost:15432/activity_db?sslmode=disable

migrate-activity: ## Apply activity-service migrations
	migrate -path services/activity-service/migrations -database "$(ACTIVITY_DB_URL)" up

run-activity:    ## Run activity-service locally (needs `docker compose up -d nats`)
	cd services/activity-service && DATABASE_URL="$(ACTIVITY_DB_URL)" go run ./cmd

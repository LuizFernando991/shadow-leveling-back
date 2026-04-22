MIGRATIONS_DIR=internal/database/migrations
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

include .env
export

build:
	@go build -o bin/gym ./cmd/api

run: build
	@./bin/gym

test/unit:
	@go test $$(go list ./... | grep -v /integration_tests) 2>&1 | grep -v "^\?" || true

test/integration:
	@go test -v ./internal/integration_tests/...

test: test/unit test/integration

infra/up:
	@docker compose up -d postgres redis

infra/down:
	@docker compose down

docker/up:
	@docker compose up -d

docker/down:
	@docker compose down

migrate/install:
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate/new:
	@test -n "$(name)" || (echo "usage: make migrate/new name=<migration_name>" && exit 1)
	@migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

migrate/up:
	@if [ -n "$(N)" ]; then \
		migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up $(N); \
	else \
		migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up; \
	fi

migrate/down:
	@if [ -n "$(N)" ]; then \
		migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down $(N); \
	else \
		migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down; \
	fi

migrate/status:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version
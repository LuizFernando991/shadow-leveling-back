MIGRATIONS_DIR=internal/database/migrations
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

include .env
export

build:
	@go build -o bin/gym ./cmd/api

run: build
	@./bin/gym

test:
	@go test -v ./...

infra/up:
	@docker compose up -d postgres

infra/down:
	@docker compose down

docker/up:
	@docker compose up -d

docker/down:
	@docker compose down

migrate/install:
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

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
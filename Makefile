include .env
export

DB_URL ?= postgres://wallet_user:wallet_pass@localhost:5432/wallet_db?sslmode=disable

.PHONY: run
run:
	go run cmd/api/main.go

.PHONY: build
build:
	go build -o bin/wallet-api ./cmd/api

.PHONY: test
test:
	go test -v -race ./...

.PHONY: docker-up
docker-up:
	docker-compose up -d --build

.PHONY: docker-down
docker-down:
	docker-compose down -v

.PHONY: db-status
db-status:
	goose -dir db/migrations postgres "$(DB_URL)" status

.PHONY: db-up
db-up:
	goose -dir db/migrations postgres "$(DB_URL)" up

.PHONY: db-down
db-down:
	goose -dir db/migrations postgres "$(DB_URL)" down

.PHONY: db-create
db-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make db-create NAME=my_migration"; exit 1; fi
	goose -dir db/migrations create $(NAME) sql


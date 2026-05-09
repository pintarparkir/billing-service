.PHONY: run build test migrate-up migrate-down docker-build

PROJECT_ENV ?= local

run:
	go run ./cmd/billing

build:
	CGO_ENABLED=0 go build -o bin/billing ./cmd/billing

test:
	go test ./...

migrate-up:
	./scripts/migrate.sh up

migrate-down:
	./scripts/migrate.sh down 1

docker-build:
	docker build -t billing-service:local .

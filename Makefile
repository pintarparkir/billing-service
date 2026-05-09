.PHONY: help run build test test-unit lint vet fmt proto \
        migrate-up migrate-down docker-build clean

PROJECT_ENV ?= local

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Run / build ────────────────────────────────────────────────────────────────
run: ## Run billing-service against local infra (postgres, rabbitmq)
	PROJECT_ENV=$(PROJECT_ENV) go run ./cmd/billing

build: ## Compile binary into bin/billing
	CGO_ENABLED=0 go build -o bin/billing ./cmd/billing

# ── Tests ──────────────────────────────────────────────────────────────────────
test: ## Run all tests (including pricing engine table-driven cases)
	go test ./...

test-unit: ## Unit tests only — pricing engine has no I/O so this is fast
	go test -short -race -count=1 ./pkg/... ./internal/...

# ── Quality ────────────────────────────────────────────────────────────────────
fmt: ## Format Go code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (requires install)
	golangci-lint run ./...

# ── Code generation ────────────────────────────────────────────────────────────
proto: ## Regenerate api/proto/billing/v1/{billing.pb.go, billing_grpc.pb.go}
	@which protoc >/dev/null || (echo "protoc not installed (brew install protobuf)" && exit 1)
	@which protoc-gen-go >/dev/null || (echo "protoc-gen-go not installed (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)" && exit 1)
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       api/proto/billing/v1/billing.proto

# ── DB migrations ──────────────────────────────────────────────────────────────
migrate-up: ## Apply DB migrations
	./scripts/migrate.sh up

migrate-down: ## Roll back one migration
	./scripts/migrate.sh down 1

# ── Container ──────────────────────────────────────────────────────────────────
docker-build: ## Build the service container image
	docker build -t billing-service:local .

# ── Housekeeping ───────────────────────────────────────────────────────────────
clean: ## Remove build artefacts
	rm -rf bin/ coverage.out

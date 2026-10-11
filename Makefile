.PHONY: migrate run test lint generate

migrate:
	@set -a; . ./.env; set +a; goose -dir migrations postgres "$$DATABASE_URL" up

run:
	@set -a; . ./.env; set +a; go run ./cmd/trip-service

test:
	@go test -race ./...

lint:
	@echo "lint: not implemented yet"

generate:
	@go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

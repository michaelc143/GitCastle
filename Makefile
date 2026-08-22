.PHONY: run test test-integration coverage fmt frontend-install frontend-test frontend-build frontend-e2e docker-up docker-down

run:
	go run ./cmd/gitcastle

test:
	go test ./...

test-integration:
	GITCASTLE_INTEGRATION=1 go test ./... -run Integration

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

fmt:
	gofmt -w ./cmd ./internal

frontend-install:
	cd frontend && pnpm install --frozen-lockfile

frontend-test:
	cd frontend && pnpm test

frontend-build:
	cd frontend && pnpm build

frontend-e2e:
	cd frontend && pnpm test:e2e

docker-up:
	docker compose up -d postgres

docker-down:
	docker compose down

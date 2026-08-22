# GitCastle

GitCastle is a Git hosting platform built from scratch.

## Current milestone

The first vertical slice provides a Go HTTP API backed by PostgreSQL and creates local bare Git repositories on disk.

### Requirements

- Go 1.23+
- Docker and Docker Compose for PostgreSQL
- Git installed locally

### Run the backend locally

```bash
cp .env.example .env
docker compose up -d postgres
go run ./cmd/gitcastle
```

The API listens on `http://localhost:8080`.

Create a repository:

```bash
curl -X POST http://localhost:8080/api/v1/repositories \
  -H 'Content-Type: application/json' \
  -d '{"owner":"alice","name":"castle"}'
```

Check health:

```bash
curl http://localhost:8080/healthz
```

### Run the frontend locally

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

The Vite app listens on `http://localhost:5173` and proxies `/api` to the Go API.

To run the complete containerized stack:

```bash
docker compose up --build
```

### Tests

```bash
go test ./...
make docker-up
make test-integration
```

The integration test uses the local PostgreSQL container and is skipped by the default unit-test command unless `GITCASTLE_INTEGRATION=1` is set.

## Roadmap

1. ✅ Local repository service
2. ✅ React/TypeScript/Vite frontend
3. ✅ Backend/frontend unit, integration, and end-to-end test harnesses
4. ✅ CI workflow, Docker images, Compose stack, and Kubernetes starter manifests
5. SSH push and clone service
6. Authentication and repository permissions
7. Issues, pull requests, and code review
8. Production secrets, TLS, backups, and isolated CI runners

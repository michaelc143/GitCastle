# GitCastle

GitCastle is a Git hosting platform built from scratch.

## Current milestone

Phase 1 (local repository service) is complete: user accounts with bcrypt-hashed
passwords and cookie sessions, bare Git repositories on disk backed by
PostgreSQL metadata, per-user repository permissions, and Git smart-HTTP push
and clone served at `/git/{owner}/{repo}.git`.

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

Register a user and log in:

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"castle-secret-1"}'

curl -c cookies.txt -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"castle-secret-1"}'
```

Create a repository (owner defaults to the logged-in user):

```bash
curl -b cookies.txt -X POST http://localhost:8080/api/v1/repositories \
  -H 'Content-Type: application/json' \
  -d '{"name":"castle"}'
```

Push and clone over Git's smart HTTP protocol:

```bash
git clone http://alice:castle-secret-1@localhost:8080/git/alice/castle.git
cd castle
git remote add origin http://alice:castle-secret-1@localhost:8080/git/alice/castle.git
git push -u origin main
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

1. ✅ Local repository service (bare repos, users, permissions, smart-HTTP push/clone)
2. ✅ React/TypeScript/Vite frontend
3. ✅ Backend/frontend unit, integration, and end-to-end test harnesses
4. ✅ CI workflow, Docker images, Compose stack, and Kubernetes starter manifests
5. Web interface: login page, repo list, file browser, commit history, diff viewer
6. Collaboration: issues, pull requests, comments, reviews, branch protection
7. Automation: webhooks, build jobs, isolated Docker runners, deployment secrets
8. Production hardening: HTTPS, rate limiting, audit logs, backups, object storage, scanning

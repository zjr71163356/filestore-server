# Repository Guidelines
## Language
使用中文回答
## Project Structure & Module Organization
- `main.go` boots the Gin server on `:8080` via `pkg/router`.
- `api/` holds HTTP handlers; `service/` orchestrates file I/O plus DB calls; `pkg/dao/` is the data access layer; `pkg/mw/` contains middleware for auth/validation; `pkg/db/` manages the MySQL connection.
- `static/` serves the upload page; `docs/` and `README.md` capture notes; `env/` contains MySQL master/slave docker-compose assets; `test/` has handler and E2E tests.

## Build, Test, and Development Commands
- `go run ./...` or `go run main.go` — start the server (requires MySQL running; see below).
- `make test` — verbose `go test ./...` across modules.
- `make test-coverage` — produces `coverage.out` with `-coverpkg=./...`.
- `make run-mysql` — starts local master/slave MySQL via `env/docker-compose.yml`; `make con-mysql` / `con-mysql2` open shells into master/slave.

## Coding Style & Naming Conventions
- Go 1.25.x module; run `gofmt` (and `goimports` if available) on all changes before sending reviews.
- Prefer context-aware functions (`ctx context.Context`) and wrap errors with `%w` for call sites to inspect.
- Keep handler/middleware validation centralized (see `pkg/mw/validation.go`); use `service` layer for orchestration and `dao` for DB access. Avoid mixing HTTP concerns inside `dao`.
- Name files and identifiers descriptively (e.g., `UploadFile`, `FileMetaUpdate`), using mixed case per Go conventions.

## Testing Guidelines
- Tests assume a reachable MySQL at `root:master_root_password@tcp(127.0.0.1:3306)/filestore`; start `make run-mysql` first. Tables are auto-created in tests, but the connection must succeed.
- Place new tests in `test/`; prefer table-driven tests and reuse helpers like `requireDB`, `randHex`, and `newTestRouter`.
- Aim to keep `make test` and `make test-coverage` green before submitting.

## Commit & Pull Request Guidelines
- Follow a Conventional Commits style (`feat:`, `fix:`, `chore:`, `env:`) consistent with existing history noted in `README.md`.
- Commits should be scoped and reversible; avoid bundling unrelated changes.
- PRs should describe the change, mention DB/test prerequisites, and include reproducible steps or curl examples for new endpoints. Link issues when applicable and attach screenshots only if UI is affected (e.g., upload page).

## Security & Configuration Tips
- Database credentials live in `pkg/db/conn.go` and `env/docker-compose.yml`; adjust via environment or `.env` overrides rather than hard-coding new secrets.
- Session cookies use the default key in `pkg/router/router.go`; rotate the secret for production and enable `Secure` if served over HTTPS.
- Uploaded files land in `./tmp`; ensure this path is writable and cleaned during tests or deployments.

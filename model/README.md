# tags-model

Tag-entity schema for the moduleforge platform. This module holds the goose
versioned migrations and sqlc-generated Go queries for the tag hierarchy:
`tags`, `entity_tags`, and related tables. It is consumed by `tags-api` to
provide type-safe database access without exposing raw SQL.

Migration range reserved for this module: **0200–0299**.

See [../docs/mf-standards/architecture/db-considerations.md](../docs/mf-standards/architecture/db-considerations.md)
for the rationale behind the Postgres + goose choices.

## Layout

- `migrations/` — goose versioned migration files (`.sql`)
- `queries/` — sqlc query files (`.sql`), one per concept
- `db/` — sqlc-generated Go code (do not edit)
- `scripts/shadow-db-lint.sh` — ephemeral-Postgres lint runner
- `sqlc.yaml` — sqlc v2 configuration

## Prerequisites

- [goose](https://github.com/pressly/goose) — `go install github.com/pressly/goose/v3/cmd/goose@latest`
- [sqlc](https://docs.sqlc.dev) — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0`
- Docker (for `make lint`'s ephemeral shadow Postgres)
- Running Postgres instance (local: `docker compose up -d` from `deploy/local/`)

## Make targets

```
make build            # generate Go from sqlc queries (default)
make gen              # same as build
make verify           # goose validate + sqlc compile
make migrate.new NAME=foo  # create a new migration file
make migrate.up       # apply pending migrations
make migrate.status   # show migration status
make test.integration # apply migrations against DATABASE_URL
make lint             # apply all migrations to an ephemeral Postgres container
make clean            # remove generated Go code
```

All targets default `DATABASE_URL` to `postgresql://tags:tags@localhost:5432/tags?sslmode=disable`.

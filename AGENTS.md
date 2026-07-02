# AGENTS.md — mod-tags

This file is the canonical reference for contributors and AI agents working on this codebase. It covers environment setup, build and test commands, project conventions, and known rough edges.

## Project overview

`@moduleforge/mod-tags` is a ModuleForge module providing tagging and labeling functionality for any resource type, anchored to core entities. It ships two sub-projects: `model/` (Go/Postgres/sqlc) and `api/` (Go HTTP). The module is designed to be mounted into applications via the ModuleForge app composition system. See the [moduleforge.module.yaml](./moduleforge.module.yaml) for the service and route configuration.

## Prerequisites

| Tool | Min version | Purpose |
|---|---|---|
| Go | 1.21+ | `model/` and `api/` sub-projects |
| Bun | 1.0+ | `gui/` sub-project (optional) |
| GNU make | 3.81+ | Build orchestration (use `gmake` on macOS if needed) |
| sqlc | latest | Go query code generation (`model/`) |
| goose | latest | Database migrations (`model/`) |

> macOS ships BSD make. Install GNU make with `brew install make` and invoke as `gmake`, or ensure `/usr/local/bin` is before `/usr/bin` in `PATH`.

## First-time setup

1. **Clone and install dependencies:**
   ```sh
   git clone git@github.com:moduleforge/mod-tags.git
   cd mod-tags
   bun install          # installs gui/ dependencies (optional if not working on gui/)
   ```

2. **Install Go dependencies** (handled by `go mod` when building; no explicit setup needed).

## Build commands

```sh
make build           # build model and api (default target)
cd model && go build ./...
cd api && go build ./...
cd gui && bun run build  # optional, only if working on GUI
```

## Test commands

```sh
make test            # run unit tests across model and api
cd model && go test ./...
cd api && go test ./...
cd gui && bun run typecheck  # optional, only if working on GUI
```

## Database migrations

Migrations are managed with goose in `model/migrations/` and run in the **200–299** range. They run automatically when the application starts. To run or roll back manually:

```sh
cd model
goose -dir migrations postgres "$DB_URL" up
goose -dir migrations postgres "$DB_URL" down
```

## Code generation (sqlc)

`model/db/` is generated from SQL in `model/queries/`. After editing a query file:

```sh
cd model && sqlc generate
```

The generated files are committed to the repo. `make clean` removes build artifacts — restore with `git checkout HEAD -- model/db/` if needed.

## Module integration

### Router mounting

The mod-tags module provides routes via `tagshttpapi.NewRouter`, which returns a full `chi.Router`. Mount it under any prefix (typically `/v1`) to yield:

- `POST /tags` — create a tag
- `GET /tags` — search tags (with authorization)
- `GET /tags/{uuid}` — retrieve a tag by UUID
- `PUT /tags/{uuid}` — replace a tag
- `PATCH /tags/{uuid}` — update a tag
- `DELETE /tags/{uuid}` — delete a tag
- `GET /entities/{uuid}/tags` — list tags for an entity

### Required dependencies

When wiring mod-tags into an application, the following services **must** be provided:

| Service | Type | Source | Purpose |
|---|---|---|---|
| `authorizer` | `authz.Authorizer` | mod-authz or mod-users | Gates every operation; non-nil error from `Authorize()` aborts the operation |
| `observerGroup` | `*observer.ObserverGroup` | assembled by compiler | Receives in-tx and post-commit notifications for mutations |
| `typeResolver` | `*types.Resolver` | mod-core | Resolves "tag" entity-type to internal type ID |
| `entityResolver` | `*entity.Resolver` | mod-core | Translates entity UUID to internal entity ID for lookups |

### ModuleForge app manifest integration

In your `moduleforge.app.yaml`, declare mod-tags as a module dependency:

```yaml
modules:
  - module: github.com/moduleforge/tags-api
    localPath: ../mod-tags
```

The moduleforge compiler will:
1. Compose all three service implementations (`tagsServices`, `tagsDeps`, router) with the above required services.
2. Mount routes under the configured prefix (default `/v1`).
3. Run migrations in order (200–299) at app startup.

See `mod-core/docs/manifest-spec.md` for the full ModuleForge manifest specification.

## Key files and directories

| Path | Purpose |
|---|---|
| `moduleforge.module.yaml` | Module configuration: services, routes, required dependencies, and migration range |
| `Makefile` | Module-level build orchestrator |
| `model/` | Postgres schema, migrations, and sqlc-generated query code |
| `model/migrations/` | goose migration files (numbered 0200–0299) |
| `model/queries/` | SQL queries consumed by sqlc |
| `model/db/` | sqlc-generated Go query code (committed; do not edit by hand) |
| `api/` | HTTP handlers, business-logic services, and route definitions |
| `api/httpapi/` | Router and handler implementations |
| `api/service/` | Tag CRUD business logic and transaction management |
| `gui/` | Optional React UI components (TypeScript/Ladle) |
| `docs/` | Project documentation and design decisions |

## Conventions

- **Internal IDs are never exposed in HTTP responses** — always use the `uuid` field.
- **Handlers are thin** — parse input, call one service method, shape response. No business logic in handlers.
- **Authorization is checked first** in every service method, before any data access.
- **Generated code (`model/db/`) is committed** and should not be edited by hand. Re-run `sqlc generate` after any query change.
- **Tag mutations are anchored to entities** — every tag is associated with a subject entity (identified by entity UUID).
- **Tags support partial mutability** — tag values can be updated, but certain fields are immutable once created (see `docs/decisions/tags-limited-immutability.md`).

## Known issues and follow-up items

See `next-steps.md` in the module root for current follow-up items and known rough edges.

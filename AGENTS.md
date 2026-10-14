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

Migrations are managed with goose in `model/migrations/`. Each module numbers its own migrations independently (no shared cross-module coordination scheme) and tracks applied state in its own dedicated `goose_db_version_tags` table, isolated from other modules. Cross-module ordering is declared via `migrations.after` in `moduleforge.module.yaml` — mod-tags declares `after: [core]` because it FKs mod-core's `entities` table. See [docs-mf-standards/manifest-spec.md](./docs/mf-standards/manifest-spec.md) §5 for the full convention. Migrations run automatically when the application starts. To run or roll back manually:

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
- `GET /tag-templates` — list tag-template catalog entries by purpose (optionally scoped to an app); an open, catalog-only read with no per-row authorization, unlike the tag routes above. Each entry also carries the purpose's read-only `oneOfDomain` flag, sourced from the admin-curated `tag_purpose_policies` registry and `false` when no policy row exists for that purpose

`RegisterRoutes` mounts `authz.RequireAuth` on an inner route group, and `NewRouter` delegates to it, so
an anonymous request gets `401` before anything is read, on both entry points. The middleware is
sudo-aware: a sudo-only identity passes the gate, and `POST /tags`, `GET /tags` and
`GET /entities/{uuid}/tags` answer it with a masked `403` (a regular actor is required). Routes the
caller registers beside `RegisterRoutes` are not gated.

Error precedence on every route: `401`, `400`, `403`, `404`, then `409`/`422` after authorization. Body
routes decode via `apiresp.DecodeJSON` with the `tags` code prefix (1 MiB cap, trailing data rejected;
details `tags.body_too_large`, `tags.invalid_json_body`); malformed UUIDs carry an `invalid_uuid` detail;
a NUL character in `purpose`/`value` is `400` `tags.nul_character`. `one_of_domain` stays `409`.
`GET /tags/{uuid}` returns `404` only after authorization, for a non-tag or deleted entity.

Error responses use the canonical **nested** envelope (`{"error": {"code", "message", "details?"}}`)
and are drawn from the reserved core error-code vocabulary (`unauthenticated`, `forbidden`,
`not_found`, `invalid_input`, `conflict`), written via `apiresp.WriteError`. UUID-keyed entity
lookups (`GET`/`PUT`/`PATCH`/`DELETE /tags/{uuid}`, `GET /entities/{uuid}/tags`) are masked by
default: a genuine miss returns `403 forbidden`, not `404 not_found`. See
[`docs/mf-standards/architecture/api-response-design.md`](./docs/mf-standards/architecture/api-response-design.md)
for the full contract; this file does not restate it.

### Required dependencies

When wiring mod-tags into an application, the following services **must** be provided:

| Service | Type | Source | Purpose |
|---|---|---|---|
| `authorizer` | `authz.Authorizer` | mod-authz or mod-users | Gates every tag CRUD/lookup operation (not the open, catalog-only `GET /tag-templates` read); non-nil error from `Authorize()` aborts the operation |
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
3. Run migrations at app startup, ordered across modules via each module's `migrations.after` declaration (mod-tags declares `after: [core]`).

See [docs/mf-standards/manifest-spec.md](./docs/mf-standards/manifest-spec.md) for the full ModuleForge manifest specification.

## Key files and directories

| Path | Purpose |
|---|---|
| `moduleforge.module.yaml` | Module configuration: services, routes, required dependencies, and cross-module migration ordering (`migrations.after`) |
| `Makefile` | Module-level build orchestrator |
| `model/` | Postgres schema, migrations, and sqlc-generated query code |
| `model/migrations/` | goose migration files, numbered independently within this module (0200–0299 today, informational only) |
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
- **Stateless request validation precedes authorization, and authorization precedes any data access** in `Create` and `Search` (a caller without permission who sends an invalid request gets `400`, not `403`); every other service method authorizes first. The one authorization exception is the open, catalog-only `GET /tag-templates` read, which intentionally makes no `Authorizer` call and is authn-only (the router still requires an authenticated caller). Exposing the `one_of_domain` flag on that route's entries is a read of existing policy data only — it introduces no authorization change and no write path.
- **Generated code (`model/db/`) is committed** and should not be edited by hand. Re-run `sqlc generate` after any query change.
- **Tag mutations are anchored to entities** — every tag is associated with a subject entity (identified by entity UUID).
- **Tags support partial mutability** — tag values can be updated, but certain fields are immutable once created (see `docs/decisions/tags-limited-immutability.md`).

## Known issues and follow-up items

See `next-steps.md` in the module root for current follow-up items and known rough edges.

- **Open security issue `p3Gt`:** `POST /tags` authorizes the caller only against the caller's own entity and never against the subject. An existing subject answers `201` (or `409` for a one-of-domain conflict) while a missing subject answers the masked `403`, so an authenticated caller allowed to create tags can probe whether an entity UUID exists and can tag an entity it cannot access. Not fixed; tracked as `p3Gt` in the project follow-ups.

## Wire changes and downstream clients

The error-order conformance work changed the wire behavior described above. Clients that may notice: app-mftodo and app-mfgit (both compose `RegisterRoutes`; they already gate with their own `requireAuth`, and in-process callers of `TagService.Create`/`Search`/`ListBySubject` now see `ErrForbidden` instead of `ErrUnauthenticated` for a sudo-only context) and `@moduleforge/tags-gui` (`TagEditor`, `mockClient.ts`). Adopting the change in the apps (pin bumps, regeneration of clients from the OpenAPI fragment) belongs to the composing apps, not this module.

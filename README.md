# @moduleforge/mod-tags

Provides tagging and labeling functionality for any resource type, anchored to core entities. The module ships a Postgres data model (`model/`), HTTP API handlers (`api/`), and optional React UI components (`gui/`) within the [ModuleForge](https://github.com/moduleforge) ecosystem.

## Installation

The module is consumed as part of a ModuleForge application. Add it to your `moduleforge.app.yaml`:

```yaml
modules:
  - module: github.com/moduleforge/tags-api
    localPath: ../mod-tags
```

For standalone Go package consumption:

```sh
go get github.com/moduleforge/tags-api
go get github.com/moduleforge/tags-model
```

## Core features

- **Tag CRUD** — create, read, update, and delete tags on any entity
- **Authorization-aware** — tag CRUD and lookup operations are gated by your app's authorization layer
- **Transactional safety** — mutations are wrapped in database transactions with observer notifications
- **Partial mutability** — tag values can be updated; label, type, and subject are immutable
- **Query flexibility** — search tags by subject, label, type, or value with authorization filtering
- **Tag-templates catalog** — an open, read-only catalog of suggested purpose/value/label/color combinations, optionally scoped to an app, for populating UI pickers; not authorization-gated beyond requiring an authenticated caller

## Quick start

For detailed build, test, and development information, see [AGENTS.md](./AGENTS.md).

```sh
make build           # build model and api
make test            # run unit tests
```

## Integration guide

Tags-module expects the following services from your app's dependency container:

- `authorizer` (`authz.Authorizer`) — gates every tag operation
- `observerGroup` (`*observer.ObserverGroup`) — notifies observers of tag mutations
- `typeResolver` (`*types.Resolver`) — resolves entity types
- `entityResolver` (`*entity.Resolver`) — translates entity UUIDs to internal IDs

Routes are mounted via `tagshttpapi.NewRouter` under your configured prefix (typically `/v1`), yielding endpoints like `/v1/tags`, `/v1/tags/{uuid}`, `/v1/entities/{uuid}/tags`, and the open, catalog-only `/v1/tag-templates`.

See [AGENTS.md](./AGENTS.md) for the complete integration guide and ModuleForge manifest specification details.

## Additional documentation

Each package and decision record has its own README or note:

- [api/README.md](api/README.md) — the `tags-api` Go service and HTTP router.
- [model/README.md](model/README.md) — the `tags-model` schema, migrations, and sqlc-generated queries.
- [gui/README.md](gui/README.md) — the `tags-gui` React component library.
- [docs/project-roadmap.md](docs/project-roadmap.md) — long-term planning: deliberately deferred, not-yet-implemented extensions beyond the active change.
- [docs/decisions/tags-limited-immutability.md](docs/decisions/tags-limited-immutability.md) — the decision record for the `tags` table's immutability policy.
- [next-steps.md](next-steps.md) — pending manual verification and deferred work after the initial implementation.
- [stories-next.md](stories-next.md) — deferred component-workbench follow-ups for the GUI package.

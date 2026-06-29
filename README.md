# @moduleforge/tags-module

Provides tagging and labeling functionality for any resource type, anchored to core entities. The module ships a Postgres data model (`model/`), HTTP API handlers (`api/`), and optional React UI components (`gui/`) within the [ModuleForge](https://github.com/moduleforge) ecosystem.

## Installation

The module is consumed as part of a ModuleForge application. Add it to your `moduleforge.app.yaml`:

```yaml
modules:
  - name: tags
    path: ../tags-module
```

For standalone Go package consumption:

```sh
go get github.com/moduleforge/tags-api
go get github.com/moduleforge/tags-model
```

## Core features

- **Tag CRUD** — create, read, update, and delete tags on any entity
- **Authorization-aware** — all operations are gated by your app's authorization layer
- **Transactional safety** — mutations are wrapped in database transactions with observer notifications
- **Partial mutability** — tag values can be updated; label, type, and subject are immutable
- **Query flexibility** — search tags by subject, label, type, or value with authorization filtering

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

Routes are mounted via `tagshttpapi.NewRouter` under your configured prefix (typically `/v1`), yielding endpoints like `/v1/tags`, `/v1/tags/{uuid}`, and `/v1/entities/{uuid}/tags`.

See [AGENTS.md](./AGENTS.md) for the complete integration guide and ModuleForge manifest specification details.

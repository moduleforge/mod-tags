# tags-api

Go service and HTTP router for mod-tags. This module exposes
tx-aware tag CRUD operations via `service` (business logic layer) and
mounts a chi subrouter for REST endpoints via `httpapi`. It depends on
`tags-model` for generated database access, and `core-api`/`core-model`
for shared entity primitives. Business logic and route implementations
land in Phase 3; this package is a clean skeleton only.

## Make targets

```
make build    # build all packages (default)
make test     # run unit tests
make lint     # go vet + gofmt check
make lint-fix # gofmt -w
make clean    # go clean ./...
```

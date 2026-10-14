# tags-api

Go service and HTTP router for mod-tags. This module exposes
tx-aware tag CRUD operations via `service` (business logic layer) and
mounts a chi subrouter for REST endpoints via `httpapi`. It depends on
`tags-model` for generated database access, and `core-api`/`core-model`
for shared entity primitives.

## Make targets

```
make build    # build all packages (default)
make test     # run unit tests
make lint     # go vet + gofmt check
make lint-fix # gofmt -w
make clean    # go clean ./...
```

## Mounting and error order

`httpapi.RegisterRoutes` mounts `authz.RequireAuth` on an inner route group and `httpapi.NewRouter` delegates to it, so an anonymous request gets `401` before the request is read. Every route then answers in the order `401`, `400`, `403`, `404`, and `409`/`422` only after authorization. See `AGENTS.md` for the details and `openapi.fragment.yaml` for the per-operation contract. Body routes cap bodies at 1 MiB and report decode failures with `tags.*` details.

# API: Tag Templates List Endpoint

## Purpose and scope

Add the open read endpoint `GET /v1/tag-templates?purpose=<p>[&scope=<app-uuid>]`
that lists rows from the `tag_templates` table created in task 001. This adds a
thin handler, a read-only service method, route registration, and the OpenAPI
fragment entry. **No `tags` handler/service code is modified beyond adding the new
route registration lines.** Invoke the standard `implement-task` procedure with
the Go Developer standard.

Depends on task 001 (the generated `ListTagTemplates` query and `tag_templates`
schema must exist).

## Requirements

### 1. Service layer — read-only list method

Add a new service surface for tag templates (either a method on the existing
`*service.Services` aggregate or a small `TagTemplateServicer` — keep it separate
from `TagServicer` so the tags CRUD paths stay untouched). The method must:

- Accept `ctx`, the core querier, the tags querier, a required `purpose string`,
  and an optional `scope *uuid.UUID` (the app's public UUID).
- **Open read: no `Authorizer` call and no access-function/`accessible_*` row
  filtering** — unlike the tags service methods. Do not gate on
  `opctx.ActorEntityID`.
- Resolve `scope` when present: translate the app UUID → internal
  `entities.id`/`apps.id` via the core querier (e.g. a `GetEntityByUUID`-style
  lookup on `coredb`). A **malformed** UUID is rejected upstream in the handler
  (400); a **well-formed but unknown** UUID resolves to no id → pass a NULL scope
  to the query so the caller still gets globals (forgiving open-read). When
  `scope` is absent, pass NULL.
- Call the generated `ListTagTemplates` query with `purpose` and the resolved
  nullable scope id.
- Map each DB row to a service/response struct that exposes **no internal ids**:
  `{Purpose, Value, Label string; Color *string; SortOrder int32; Scope *uuid.UUID}`
  where `Scope` is the row's `scope_uuid` (nil for globals).

Follow the existing `service` package conventions (see `service/tag.go` and
`service/service.go`) for querier plumbing and the `pgtype` ↔ Go-type mapping.

### 2. Handler — `api/httpapi`

Add a thin handler (e.g. in a new `api/httpapi/tag_templates.go`) for
`GET /tag-templates`:

- Read `purpose` from the query string; **required** — empty/absent →
  `apiresp.WriteError(w, r, fmt.Errorf("%w: purpose is required", apiresp.ErrInvalidInput))`.
- Read optional `scope`; if present, parse as `uuid.UUID` — malformed →
  `apiresp.ErrInvalidInput` (400).
- Call the service method; on error `apiresp.WriteError(w, r, err)`.
- On success, write a **bare JSON array** (flat-family convention per
  `api-response-design.md`) via `apiresp.WriteJSON(w, http.StatusOK, resp)` where
  each element is `{purpose, value, label, color, sortOrder, scope}` (JSON keys
  camelCase per the existing `tagResponse`); `scope` is the app UUID string or
  `null`. No internal `id` is emitted.

Keep the handler thin (parse → one service call → shape response), consistent with
the module's thin-handler convention.

### 3. Route registration — `api/httpapi/router.go`

Register `r.Get("/tag-templates", h.handleListTagTemplates)` in **both**
`NewRouter` and `RegisterRoutes` (the manifest wires routes via
`RegisterRoutes`). The route is thereby mounted under the app manifest's existing
`scope: authenticated` group alongside the tag routes — "open" here means no
per-row authz, not a new unauthenticated surface. (Whether the endpoint should be
truly public/unauthenticated is a flagged open question; if so, a separate
public route group / manifest entry would be needed — out of scope unless the
manager confirms.)

### 4. OpenAPI fragment — `api/openapi.fragment.yaml`

Add the `/tag-templates` path (operationId e.g. `listTagTemplates`, `purpose`
required query param, optional `scope` uuid query param, `200` → array of a new
`TagTemplate` schema, `400` → `BadRequest`) and the `TagTemplate` component schema
(`purpose, value, label, color, sortOrder, scope`). Follow the existing fragment
style.

## Validation

- `cd api && go build ./...` and `cd api && go test ./...` pass.
- New handler test(s) in `api/httpapi` (follow `handlers_test.go` / `mock_test.go`
  patterns) cover: missing `purpose` → 400; `purpose` only → globals-only result;
  `purpose` + valid `scope` → globals + scoped result; malformed `scope` → 400.
- `git diff` shows the tags handler/service logic unchanged except for the two
  added route-registration lines in `router.go`; `service/tag.go` create/update/
  delete paths untouched.
- The endpoint returns a bare JSON array and never emits an internal integer id.
- Error responses use the nested `apiresp` envelope with reserved codes.
- OpenAPI fragment still parses (e.g. `cd api && make` lint target if present /
  redocly validation).

## Metadata

architectural_impact: true

## References

- [Schema grounding notes](../notes/schema-grounding.md) — endpoint behavior,
  scope resolution, open-read decision, response shape.
- `api/httpapi/tags.go` — handler patterns (`apiresp.WriteError`/`WriteJSON`,
  query parsing, UUID parsing, `tagResponse` shape).
- `api/httpapi/router.go` — `NewRouter` + `RegisterRoutes` (register the new route
  in both).
- `api/service/service.go`, `api/service/tag.go` — service aggregate, querier
  plumbing, `pgtype` mapping.
- `docs/mf-standards/architecture/api-response-design.md` — bare-array success
  body, nested error envelope, reserved error codes.
- `moduleforge.module.yaml` — routes block; `RegisterRoutes` under
  `scope: authenticated`.

## Checkpoint hints

- After adding the service method and its unit coverage.
- After adding the handler + route registration and handler tests.
- After updating the OpenAPI fragment.
</content>

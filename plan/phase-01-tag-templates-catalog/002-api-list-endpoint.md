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

## Status

**Outcome:** succeeded. Date: 2026-07-18.

**Implementation summary.** Added `service.TagTemplateServicer` /
`TagTemplateService` (`api/service/tag_template.go`) as a new, stateless
member of the `*service.Services` aggregate (`api/service/service.go`),
kept fully separate from `TagServicer`. `List` validates `purpose`
(required, trimmed), resolves an optional `scope` app UUID to an internal
`apps.id`/`entities.id` via `coreQ.GetEntityByUUID` — a well-formed but
unknown UUID falls through to a NULL scope param (forgiving open-read) per
the task's spec, rather than erroring — then calls the generated
`ListTagTemplates` query and maps rows to `TagTemplate` (no internal ids).
Added the thin handler `handleListTagTemplates`
(`api/httpapi/tag_templates.go`, `GET /tag-templates`): required `purpose`
query param (400 if absent), optional `scope` query param parsed as UUID
(400 if malformed), one service call, bare-JSON-array response via
`apiresp.WriteJSON`. No actor check, no authz call — an intentional open
read, unlike the tags handlers. Registered `r.Get("/tag-templates",
h.handleListTagTemplates)` in both `NewRouter` and `RegisterRoutes`
(`api/httpapi/router.go`) — the only lines touched in that file. Added the
`/tag-templates` path and `TagTemplate` schema to
`api/openapi.fragment.yaml`, validated with `redocly lint` (one pre-existing,
unrelated `info-license` warning; no errors).

**Environment workaround (not a code change).** As in task 001, this
worktree's nested path breaks the `api/go.mod` replace directives
(`../../mod-core/api`, `../../mod-core/model`), which resolve relative to
the worktree root rather than the real sibling `mod-core/` checkout. `cd api
&& go build ./...` / `go test ./...` fail out of the box with "replacement
directory ... does not exist." Worked around for every build/test/vet
invocation in this task by setting `GOWORK` to a scratch `go.work` file (not
committed, not under `worktree` or `plan/`) that `use`s this worktree's
`api/` and `model/` and `replace`s `core-api`/`core-model` with absolute
paths to the real `/Users/zane/playground/moduleforge/mod-core/{api,model}`
checkout. From the actual `mod-tags` checkout (a real sibling of
`mod-core/`), the committed replace directives resolve correctly on their
own — no `go.mod` change is needed or was made.

**Same-diff self-fixes (pre-existing compile breaks, not part of this
task's feature work — bucket 2, recorded per SKILL contract).** With the
`GOWORK` workaround in place, `api/service` and `api/httpapi` failed to
compile even on the pristine worktree, before any edit of mine, via two
independent, unrelated causes:
1. Task 001 added `ListTagTemplates` to the generated `tagsdb.Querier`
   interface (in `tags-model`, a legitimate, expected consequence of that
   task). `api/service/mock_test.go`'s `mockTagQuerier` and
   `api/service/display_test.go`'s `singleTagQuerier` — pre-existing test
   doubles the api-side tags tests already depend on — no longer satisfied
   the interface. Added a `ListTagTemplates` method to both (functional on
   `mockTagQuerier`, backing this task's own service-level tests; a no-op
   stub on `singleTagQuerier`, matching its existing single-purpose-double
   pattern).
2. Independently, the real sibling `mod-core` checkout's `coredb.Querier`
   interface has grown apps-CRUD methods (`GetAppBySlug`, `GetAppByUUID`,
   `InsertApp`, `ListApps`, `UpdateApp`) from unrelated, separately-landed
   `mod-core` work (the same `apps` table this task's own scope-resolution
   logic reads via `GetEntityByUUID`). `api/service/mock_test.go`'s
   `mockCoreQuerier` and `api/httpapi/mock_test.go`'s `fakeCoreQuerier` —
   both files already touched for point 1 / for the new
   `fakeTagTemplateService` — no longer satisfied `coredb.Querier`. Added
   no-op stubs for all five methods to both, mirroring each file's existing
   not-implemented-method pattern (e.g. `CreateLegalEntity`).

Both fixes are purely mechanical interface-satisfaction stubs (zero
behavior change to any tags code path) inside files already part of this
task's diff for its own feature work, and were necessary for the task's
required `go build ./...` / `go test ./...` validation to pass at all.

**Validation.** `cd api && go build ./...`, `go vet ./...`, `gofmt -l .`
(clean), and `go test ./...` (all packages, all pre-existing tags tests plus
14 new tag-template tests) — all green under the `GOWORK` workaround above.
`git diff` confirms `api/httpapi/tags.go`, `api/httpapi/subject_tags.go`,
`api/service/tag.go`, and `api/service/errors.go` are byte-identical to
the pre-task state; `api/httpapi/router.go` gained exactly the two route
registration lines; `api/service/service.go` gained only the
`TagTemplate` aggregate field/wiring (not `tag.go`).

**Flagged (open questions from the task doc, not decided here).** Per the
task doc's own inline flags: (1) whether `/tag-templates` should be a truly
public/unauthenticated route (separate manifest route group) rather than
sitting behind the shared `scope: authenticated` group it inherits today by
being registered via `RegisterRoutes`; (2) whether `?scope=<uuid>` should
return scoped-only rows instead of globals+scoped. Neither was re-decided;
both are exactly as the task doc left them.

**Files touched:**
- `api/service/tag_template.go` (new)
- `api/service/tag_template_test.go` (new)
- `api/service/service.go` (aggregate wiring only)
- `api/service/mock_test.go` (added `ListTagTemplates` to `mockTagQuerier`;
  added apps-CRUD stubs to `mockCoreQuerier`)
- `api/service/display_test.go` (added a `ListTagTemplates` stub to
  `singleTagQuerier`)
- `api/httpapi/tag_templates.go` (new)
- `api/httpapi/tag_templates_test.go` (new)
- `api/httpapi/mock_test.go` (added `fakeTagTemplateService`; added
  apps-CRUD stubs to `fakeCoreQuerier`; added `buildTestDepsForTemplates`
  and wired `TagTemplate` into `buildTestDeps`)
- `api/httpapi/router.go` (two route-registration lines)
- `api/openapi.fragment.yaml` (new `/tag-templates` path + `TagTemplate`
  schema)
- `plan/phase-01-tag-templates-catalog/002-api-list-endpoint.md` (this
  file — status only)
</content>

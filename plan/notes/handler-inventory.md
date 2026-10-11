# Handler Inventory

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Planner findings for the mod-tags share of `tasks-tags-conformance`: how task worktrees build, the planning decisions every task follows, who is affected, and a preliminary per-route table. Task 7.1 verifies every row against source and appends the authoritative five-axis audit table ([common section 2](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#2-per-handler-conformance-audit-procedure)) under [Audit table](#audit-table). Where source disagrees with this note, the source wins: the task records the difference and does not change a decision on its own. Line numbers are from plan-branch base `4e58869`.

Shared conventions live in the wave's [common conformance instructions](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md) ("common section N", "common rule N").

## Building and validating from a task worktree

`api/go.mod` resolves mod-core through relative replaces (`../../mod-core/api`, `../../mod-core/model`); `tags-model` is the intra-repo `../model`. From a task worktree at `mod-tags/worktrees/task/<name>/`, `../../mod-core` is `mod-tags/worktrees/task/mod-core`, a symlink to `/Users/zane/playground/moduleforge/mod-core` (mod-core `main`). It already exists. If it is missing, run `make preflight.siblings` from the worktree root. Check resolution with:

```bash
cd <worktree>/api
go list -m -f '{{.Dir}}' github.com/moduleforge/core-api   # expect .../worktrees/task/mod-core/api
```

Fallback, only if the symlink cannot be made to resolve: point `GOWORK` at a `go.work` kept OUTSIDE the repository (the same recipe the planner verified for mod-tasks). Never commit a `go.work`, never edit `go.mod`/`go.sum`:

```bash
mkdir -p "$SCRATCH/gw" && cat > "$SCRATCH/gw/go.work" <<EOF
go 1.26.2

use (
	<worktree>/api
	<worktree>/model
)

replace github.com/moduleforge/core-api v0.0.0 => /Users/zane/playground/moduleforge/mod-core/api
replace github.com/moduleforge/core-model v0.0.0 => /Users/zane/playground/moduleforge/mod-core/model
EOF
cd <worktree>/api && GOWORK="$SCRATCH/gw/go.work" go test ./...
```

`gui/` dependencies are not installed in task worktrees and this plan changes no GUI code, so do not run the root `make build`/`make test`. Standard per-task validation, from `api/`:

```bash
test -z "$(gofmt -l .)" || gofmt -l .
go vet ./...
go vet -tags=integration ./...
go test ./...
```

Integration suites: `api/service/*_integration_test.go` (`//go:build integration`; grants, one-of-domain, tag-template upsert). Run `go test -tags=integration -p 1 -count=1 -v ./service/...` from `api/`. They dial port 5432 (user/password `users`, host override `TAGS_DEV_PG_HOST`) and drop/recreate only their own verify databases. If no Postgres is reachable they print `integration: skipping ...` and exit 0: report that as skipped, never as passed. Do not start or stop shared containers; a throwaway container is acceptable only if port 5432 is free (`docker run -d --rm --name tasks-tags-it-$$ -e POSTGRES_USER=users -e POSTGRES_PASSWORD=users -p 5432:5432 postgres:16`, then `docker rm -f` it).

Baseline at plan start (verified by the planner): `go build ./...` passes and `gofmt -l .` is clean, but `go vet ./...` (with and without `-tags=integration`) and `go test ./...` FAIL to build `api/httpapi` and `api/service`. `httpapi/mock_test.go:157` (`fakeCoreQuerier`) and `service/mock_test.go:407` (`mockCoreQuerier`) lack 11 `coredb.Querier` methods: `GetFieldCryptoKeyByVersion`, `InsertActiveFieldCryptoKey`, `InsertInitialFieldCryptoKey`, `ListFieldCryptoKeyMetadata`, `ListUsableFieldCryptoKeys`, `MarkFieldCryptoKeyCompromised`, `RetireActiveFieldCryptoKey`, `SetFieldCryptoKeyDecryptableUntil`, `UpdateCorporationEINBlob`, `UpdateNaturalPersonSSNBlob`, `UpdateServiceAccountLabel`. With stubs supplied through `go vet -overlay`, all three commands pass. Task 7.2 fixes this.

## Planning decisions

Tasks implement these. If the source contradicts a decision's premise, halt and report.

- **D1 RequireAuth placement.** `RegisterRoutes` (`api/httpapi/router.go:50`) registers onto a caller-supplied router (the shared `/v1` router in app-mftodo and app-mfgit), so wrap its registrations in `r.Group(func(r chi.Router) { r.Use(authz.RequireAuth); ... })` and never call a bare `r.Use` on the router passed in (common section 3). `NewRouter` (`:28`) duplicates the 8 registrations today instead of delegating: change it to `r := chi.NewRouter(); RegisterRoutes(r, d); return r`, so the gate cannot drift between the two entry points. All 8 routes are authenticated-class. The manifest route already sets `scope: authenticated`. Both apps also wrap `RegisterRoutes` in their own `requireAuth` group (app-mftodo `cmd/server/main.go:306`, app-mfgit `:328`).
- **D2 Inline 401 removal.** 8 sudo-unaware handler checks: `tags.go` lines 55, 97, 154, 189, 251, 295; `subject_tags.go:17`; `tag_templates.go:48`. They are deleted only after task 7.3's mount is merged and its tests prove coverage (common rule 1). The existing handler tests already use `NewRouter` (`handlers_test.go`, `tag_templates_test.go`), so their `*_401_Unauthenticated` tests keep passing through the middleware once `NewRouter` delegates. `GET /tag-templates` stays authentication-only: no `Authorize` call is added.
- **D3 Sudo-aware actor guards.** Three service methods read the sudo-unaware regular actor id and use it as owner or row scope: `TagService.Create` (`tag.go:138`, owner), `TagService.Search` (`:313`, row scope), `TagService.ListBySubject` (`:417`, row scope). **Decision** (same as mod-tasks D3 and the sibling `workflows-conformance` D3): the 401 comes from `authz.RequireAuthenticated(ctx)`; the owner/scope id stays `opctx.ActorEntityID`; if that is absent while the context is authenticated (sudo-only), return `ErrForbidden`. Count: 8 handler checks + 3 service guards = 11 (the manifest's "~11").
- **D4 Existing masking stays; no `AuthorizeResolve` conversion.** `GetByUUID`, `UpdateByUUID`, `UpdateTagValue` and `DeleteByUUID` resolve with `entityResolver.Resolve(ctx, coreQ, uuid, "tag")` (default policy: a miss is `ErrForbidden`) before authorizing. `Create` and `ListBySubject` resolve the subject with the separate slug `"subject_entity"` the same way. No `AllowNotFound` call exists in this module or either composing app (planner grep). `Search` turns an unknown `owner`/`subject` filter UUID into an empty list (`tag.go:353-368`), which reveals nothing. `GetByUUID`'s `ErrNotFound` (`:284`) fires only after authorization, for a resolved entity with no `tags` row (a consistency case); it is a legitimate post-authorization 404 that task 9.1 confirms. These paths add no target-0 `Authorize` call, so common rule 5 does not bite in mod-tags.
- **D5 `DecodeJSON` options.** `POST /tags` (`tags.go:60`), `PUT /tags/{uuid}` (`:200`), `PATCH /tags/{uuid}` (`:262`) use `apiresp.DecodeJSON(w, r, &req, apiresp.WithCodePrefix("tags"))`. They are strict already (`DisallowUnknownFields`), so there is no `WithAllowUnknownFields`. There is no empty-body special case: each route already rejects an empty body. Today these routes read the body WITHOUT any size cap; the default 1 MiB cap is new. Today every 400 from these handlers is a bare `invalid_input` with the fixed per-code message and NO `details` (`apiresp.WriteError` never echoes the wrapped text), so `DecodeJSON` changes only the `details` array, which is newly present (`tags.invalid_json_body`, `tags.body_too_large`). The prefix `tags` follows the `<resource>.<reason>` convention of the sibling modules. The handler-level checks after decode (subject UUID, PUT color required/type, PATCH value required) stay where they are: stateless and before the service.
- **D6 `ParseUUIDParam` adopted.** Unlike mod-tasks (prefixed codes on the wire), mod-tags' UUID failures carry no details today, so `apiresp.ParseUUIDParam` only adds a `details` entry (`invalid_uuid`, with the field name) and renames nothing. Use it for the path `{uuid}` on the 5 routes that have one (`GET`/`PUT`/`PATCH`/`DELETE /tags/{uuid}`, `GET /entities/{uuid}/tags`), and for the UUID query parameters `owner`/`subject` (`GET /tags`) and `scope` (`GET /tag-templates`) and the body field `subject` (`POST /tags`), each with its own field name. This is additive; the top-level code stays `invalid_input`.
- **D7 Stateless checks before authorization.** `TagService.Create` authorizes (`tag.go:154`) BEFORE purpose/value/color validation (`:159-175`); `TagService.Search` authorizes (`:327`) BEFORE "at least one of owner or subject is required" (`:331`). Move each block ahead of the authorize call, after D3's actor guard. There is no handler-side fetch before these service calls, so the service reorder is sufficient and no exported validator is needed. Already stateless-first (verify only): `UpdateByUUID` color (`:489`), `UpdateTagValue` value (`:583-589`), `TagTemplateService.List` purpose (`tag_template.go:91`).
- **D8 409/422 classification (planned; task 9.1 decides).** `one_of_domain`: the `tags_enforce_one_of_domain` trigger (`model/migrations/0205_tags_one_of_domain.sql`) raises SQLSTATE 23505, which `Create` maps to `ErrConflict` (409, "tag already exists", `tag.go:237`) inside the transaction, after authorization. Planned: **keep 409**. It is a uniqueness conflict with an existing tag (the rule's 409 case), not a reference to a value outside a registry (422). The GUI's `TagEditor` handles it as a 409 (`gui/src/TagEditor.tsx:239`). After 0205 no other unique index exists on `tags` (the primary key is a freshly inserted entity id), so the bare 23505 mapping cannot misfire today; the trigger raises without a constraint name, so the mapping cannot be narrowed to a constraint (unlike mod-core's `apps_slug_key` fix). Task 9.1 records this. Other post-authorization errors: `Create`'s internal owner-entity `ErrNotFound` (`:191`, internal consistency, unchanged) and D4's `GetByUUID` 404.
- **D9 Order-test harness (approach B).** As in mod-tasks D10: the order tests live in `api/service` as the external test package `service_test` and drive the real `httpapi.NewRouter` over the real services. A test-only `api/service/export_test.go` exposes a constructor with injected tx-querier factories (`newCoreQuerier`, `newTagQuerier`) and aliases for the existing fakes (`fakeDB`, `mockCoreQuerier`, `mockTagQuerier`, `authzFunc`, `stubOpResolver`, `recordingObserver`). Fallback: tests in `api/httpapi` over a fake `txhelper.DB`. Prefixes: shared `order`, routes `tgOrd`.
- **D10 No `AuthzFirst`.** No `AuthorizeType` call exists; the own-entity authorizations of `Create`/`Search` are target-aware.
- **D11 Sequencing and prefixes.** Same-package tasks never run in parallel. Prefixes: `tgAuthn` (7.3), `tgSudo` (7.4), `tgInl` (7.5), `tgShape` (8.1), `tgConf` (9.1), `order`/`tgOrd` (9.2).

## Consumers affected

- **app-mftodo and app-mfgit** compose this module (`tagshttpapi.RegisterRoutes`, inside their own `requireAuth` groups). Their `cmd/server/go.mod` files replace `tags-api` with the relative sibling `../../../mod-tags/api`, so local app builds see `main` as soon as it merges; their `versions.lock.yaml` pins `mod-tags` at `0465080`. Pin bumps and adoption belong to the mftodo-mfgit plan-group; this plan only writes wire-risk notes.
- **`@moduleforge/tags-gui`** (this repo's `gui/`): `TagEditor` maps 409 to its one-of-domain banner and binds `invalid_input` details; the new `details` entries are additive.
- **In-process callers:** service tests and the integration tests observe D3 (sudo-only `ErrForbidden`) and D7 (validation before authorization).

## Preliminary route table

"Today" describes base `4e58869`. Authn today is the inline `opctx.ActorEntityID` check on every route (D2).

| # | Route | Handler | Stateless checks today | Lookup / authorize | Post-authorization errors | Tasks |
|---|---|---|---|---|---|---|
| 1 | POST /tags | `handleCreateTag` `tags.go:54` | strict uncapped decode; body `subject` UUID; purpose/value/color in service AFTER authorize | service guard; `create` on own id; subject `Resolve` (masked) | 409 one_of_domain / duplicate; owner-entity 404 (internal) | 7.3-7.5, 8.1, 9.1, 9.2 |
| 2 | GET /tags | `handleSearchTags` `:96` | `owner`/`subject` UUIDs; "owner or subject required" in service AFTER authorize | service guard; `list` on own id | none (unknown filter UUID gives empty list) | 7.3-7.5, 8.1, 9.2 |
| 3 | GET /tags/{uuid} | `handleGetTag` `:153` | uuid | `Resolve` "tag" (masked); `read` | 404 missing tags row (consistency) | 7.3, 7.5, 8.1, 9.1, 9.2 |
| 4 | PUT /tags/{uuid} | `handlePutTag` `:188` | uuid; strict uncapped decode; color required/type (handler); color format (service, before resolve) | `Resolve` (masked); `update` | none (in-tx miss masked) | 7.3, 7.5, 8.1, 9.2 |
| 5 | PATCH /tags/{uuid} | `handlePatchTag` `:250` | uuid; strict uncapped decode; value required (handler); value trim/length (service, before resolve) | `Resolve` (masked); `update` | none (in-tx miss masked) | 7.3, 7.5, 8.1, 9.2 |
| 6 | DELETE /tags/{uuid} | `handleDeleteTag` `:294` | uuid | `Resolve` (masked); `delete` | none (in-tx miss masked) | 7.3, 7.5, 8.1, 9.2 |
| 7 | GET /entities/{uuid}/tags | `handleSubjectTags` `subject_tags.go:16` | uuid; `purpose`; pagination | service guard; subject `Resolve` (masked); `list` on subject | none | 7.3-7.5, 8.1, 9.2 |
| 8 | GET /tag-templates | `handleListTagTemplates` `tag_templates.go:47` | `purpose` required; `scope` UUID | none (authn-only by design); unknown scope falls back to globals | none | 7.3, 7.5, 8.1, 9.2 |

## Audit table

Task 7.1 appends the authoritative five-axis table here.

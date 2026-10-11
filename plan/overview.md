# Overview

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Bring every mod-tags HTTP handler (`api/httpapi/*`), and the service methods beneath it (`api/service/*`), into conformance with the HTTP error-precedence rule:

1. 401 (unauthenticated);
2. 400 (stateless request shape: path UUID parse, body decode and shape);
3. 403;
4. 404.

State-dependent 409/422 come only after authorization. This is the mod-tags share of the federated plan `tasks-tags-conformance` (plan-group "mod-tasks and mod-tags handler conformance", wave program `http-error-precedence-conformance`, wave 2). The mod-tasks share uses the same plan slug and branch name (`plan/tasks-tags-conformance`) in the mod-tasks repo, with its own `plan/` (phases 01-06). The two modules do not depend on each other, so the shares are independent and may run concurrently.

The shared, plan-independent instructions live in the wave's [common conformance instructions](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md) on mod-core `main`. Task documents cite it as "common section N" / "common rule N" and do not restate it. The normative spec is [http-error-precedence.md](/Users/zane/playground/moduleforge/docs-mf-standards/architecture/http-error-precedence.md) (read the sibling `docs-mf-standards` checkout; the submodule is empty in worktrees). Lessons from the completed sibling plan are in its [plan summary](/Users/zane/playground/moduleforge/mod-core/plan/plan-summary-core-handler-conformance.md).

In scope (wave manifest reserved-slot description, verified against source in the [handler inventory](./notes/handler-inventory.md)):

- mount `authz.RequireAuth` inside `RegisterRoutes` (and make `NewRouter` delegate to it), then remove the inline 401 checks: 8 sudo-unaware `opctx.ActorEntityID` checks in the handlers plus 3 sudo-unaware service-layer guards (the manifest's "~11");
- `GET /tag-templates` stays authentication-only: it gets `RequireAuth` like every route but no authorization call;
- move the 3 request-body decode sites to `apiresp.DecodeJSON` (today they have no body cap at all) and adopt `apiresp.ParseUUIDParam`;
- run stateless validation before authorization in `TagService.Create` and `TagService.Search`;
- audit `one_of_domain` (and every other post-authorization error) as 409 or 422;
- order tests with `apiresp/ordertest` for all 8 routes;
- wire-risk notes for the composing apps (app-mftodo, app-mfgit).

Out of scope:

- GUI code (`gui/`);
- `go.mod`/`go.sum` edits, `versions.lock.yaml` pin bumps in the apps, app regeneration, and any push (common rules 7-8; adoption belongs to the mftodo-mfgit plan-group);
- changing owner/row-scoping semantics under impersonation (D3);
- converting the already-masked `entityResolver.Resolve` lookups to `AuthorizeResolve` (D4);
- the stale `plan/notes/tag-templates-response-shape.md` left on `main` by an earlier plan (finding `5GX9`; not edited here);
- the byte-vs-character length mismatch in `TagService` purpose/value validation (finding `BTWb`).

## Current status

Plan created; no task has run. Phase 07 starts first: tasks 7.1 (read-only audit) and 7.2 (test-build fix) can run in parallel.

Pre-conditions found during planning:

- The wave-1 helpers are present on mod-core `main` (verified at mod-core `5d3dfe9`). Task worktrees build against the local mod-core checkout through relative replaces and the `worktrees/task/mod-core` symlink; see [building from a task worktree](./notes/handler-inventory.md#building-and-validating-from-a-task-worktree).
- Baseline is red: `go vet ./...` and `go test ./...` in `api/` fail to build `api/httpapi` and `api/service`, because both `coredb.Querier` test fakes lack the same 11 methods mod-core `main` added. The planner confirmed with a `go vet -overlay` that the 11 stubs make `vet`, `vet -tags=integration` and `test` green. Task 7.2 fixes this.
- No existence leak was found: every UUID lookup already masks a miss as 403 (D4), so this share has no existence-leaks phase. Task 7.1 confirms.

## Overview

Success criteria:

- every one of the 8 routes returns the rule's order;
- no caller who cannot access a tag or subject entity can tell a missing id from an existing one, including through body validation;
- success shapes and existing error `code` values are unchanged except for the recorded wire risks;
- `gofmt`, `go vet` (with and without `-tags=integration`), `go test ./...` pass, and the integration suites pass or skip cleanly with the skip reported;
- `ordertest` covers every authorized route; `GET /tag-templates` (authn-only) has a dedicated order test.

Hard constraints:

- common rule 1 (never delete a check before its replacement is in place and tested);
- `service.New`, `httpapi.NewDeps`, `httpapi.NewRouter` and `httpapi.RegisterRoutes` keep their signatures;
- the GUI's `TagEditor` relies on 409 "tag already exists" for one-of-domain conflicts (D8);
- no pushes, pin bumps, lockfile or `go.mod` edits (common rule 8).

The planner's findings, the 8-row preliminary route table and decisions D1-D11 are in the [handler inventory](./notes/handler-inventory.md). The planned wire-visible changes are in the [wire-risk note](./notes/wire-risk.md).

Phase numbers continue the federated plan's global sequence (mod-tasks holds 01-06).

1. **`authenticate-routes`** (phase 07)
   1. `001-audit-handlers` (sonnet-low): read-only five-axis audit of the 8 routes; completes the audit table.
   2. `002-restore-test-build` (haiku-med): the 11 missing `coredb.Querier` stubs in both fakes. Parallel with 7.1.
   3. `003-mount-require-auth` (sonnet-med): `NewRouter` delegates to `RegisterRoutes`; `RequireAuth` inside `r.Group` (D1); 401-before-400 tests for all 8 routes. After 7.2.
   4. `004-service-actor-guards` (sonnet-med): sudo-aware 401 plus sudo-only 403 in `Create`, `Search`, `ListBySubject` (D3). After 7.2; `api/service` only, so parallel with 7.3.
   5. `005-remove-inline-authn-checks` (haiku-med): delete the 8 inline checks; sudo-only tests. After 7.3 and 7.4.
2. **`decode-and-shape`** (phase 08)
   1. `001-tag-handlers-decodejson` (sonnet-med): 3 decode sites to `DecodeJSON` (prefix `tags`), `ParseUUIDParam`, and stateless-before-authorize in `Create`/`Search`.
3. **`conflicts-and-order-tests`** (phase 09; sequential)
   1. `001-state-conflict-audit` (sonnet-med): one_of_domain 409-vs-422 and every other post-authorization error (D8).
   2. `002-order-tests` (sonnet-med): harness plus the 7 tag routes and the `/tag-templates` authn-only test.
4. **`wire-docs-and-validation`** (phase 10; sequential)
   1. `001-wire-risk-docs` (sonnet-low).
   2. `002-full-validation` (sonnet-low).
5. **`doc-updates`** (phase 11): `001-update-architecture-docs` (sonnet-high). Registered by the architectural-implications check: the plan changes documented API error behavior (`AGENTS.md` "Router mounting" and "Conventions").

Parallel-eligible: 7.1 with 7.2; 7.3 with 7.4. Every mod-tags task may also run concurrently with any mod-tasks task. Everything else within mod-tags is sequential.

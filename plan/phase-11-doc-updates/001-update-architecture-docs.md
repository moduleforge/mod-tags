# Update Architecture Docs

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Review and update mod-tags' architecture-level and decision docs so they reflect the error-precedence conformance changes planned in this session:

- `authz.RequireAuth` inside `RegisterRoutes`, with `NewRouter` delegating to it, and `GET /tag-templates` authenticated but not authorized;
- stateless request shape (strict, capped `DecodeJSON`; `ParseUUIDParam`; `Create`/`Search` validation) before authorization;
- the sudo-aware 401 and sudo-only 403 in `Create`, `Search`, `ListBySubject`;
- the existing masked-miss 403s (unchanged, now documented as the precedence rule's 403 stage);
- the one-of-domain conflict confirmed as 409 after authorization, as task 9.1 decided it.

mod-tags has no `docs/architecture.md` and no `docs/*-spec.md`; its architecture-level record is `AGENTS.md` ("Router mounting", "Conventions"), `docs/decisions/` and `docs/project-roadmap.md`.

## Requirements

- The planned implementation task documents that surfaced the architectural implications. These are plain paths relative to the plan worktree; they will be complete when this phase runs:
  - `plan/phase-07-authenticate-routes/003-mount-require-auth.md`
  - `plan/phase-07-authenticate-routes/004-service-actor-guards.md`
  - `plan/phase-08-decode-and-shape/001-tag-handlers-decodejson.md`
  - `plan/phase-09-conflicts-and-order-tests/001-state-conflict-audit.md`
- Architecture, decision and spec files to review:
  - `AGENTS.md` "Router mounting" and "Conventions" (task 10.1 updated the wire text; fix anything still architecturally false, do not duplicate it).
  - `docs/decisions/tags-one-of-domain.md`: its enforcement section says the trigger's 23505 maps to 409; confirm it still holds and add one sentence that the 409 is a post-authorization error under the precedence rule, if the record describes HTTP behavior.
  - A new short decision record (e.g. `docs/decisions/http-error-precedence-conformance.md`) in the style of the existing ones: the rule as applied to mod-tags, the sudo-only 403 choice, the `tags` decode prefix and `ParseUUIDParam` adoption, the 409 decision for one-of-domain and why not 422. Link the canonical spec (`docs/mf-standards/architecture/http-error-precedence.md`) rather than restating it.
  - `README.md`, `api/README.md`, `docs/project-roadmap.md`: review; edit only where false.
  - The shared `docs/mf-standards/architecture/http-error-precedence.md` lives in `docs-mf-standards`; do not edit it. If it states something this plan made false, file a followup with `target_project` set to `docs-mf-standards`.
- role_doc: `plugins/flow/roles/architect-backend.md`
- Procedure: `plugins/flow/task-procedures/update-architecture-docs/SKILL.md`.
- The architect writes durable content into the project docs listed above and never links the plan task docs listed in these Requirements. Those paths are plain code text, not Markdown links.

## Validation

- Each file named above was reviewed. The report lists, per file, "updated" (with a summary) or "already correct".
- `grep -rn '](.*plan/' README.md AGENTS.md api/README.md docs/` shows no links into `plan/` other than the durable artifacts the ephemeral linking rule allows.
- If any Go package doc was edited, from `api/`: `test -z "$(gofmt -l .)" && go vet ./... && go test ./...`.

## Metadata

architectural_impact: false

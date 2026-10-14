# Wire Risk Docs

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

Record every wire-visible change this plan made in the mod-tags docs that describe the API (common rule 4; "Docs" item of [common section 6](/Users/zane/playground/moduleforge/mod-core/plan/waves/http-error-precedence-conformance/common-conformance-instructions.md#6-standard-per-plan-task-template)). Source of truth: the [wire-risk note](../notes/wire-risk.md) as corrected under "Recorded by tasks". Docs only.

## Requirements

1. `api/openapi.fragment.yaml`: an "Error precedence" description (401, 400, 403, 404; 409/422 after authorization); per operation, 401 before 400, the new 1 MiB cap and `tags.*` decode details on the body routes, `invalid_uuid` details, the sudo-only 403 on `POST /tags`, `GET /tags`, `GET /entities/{uuid}/tags`, and the unchanged 409 for one-of-domain.
2. `AGENTS.md` "Router mounting" (the error-envelope paragraph at about line 87) and "Conventions" (the "Authorization is checked first" bullet: stateless shape now precedes authorization; `GET /tag-templates` remains authn-only).
3. `README.md` and `api/README.md`: a short note on the error order and `RequireAuth` inside `RegisterRoutes`, where they describe mounting or errors.
4. Name the downstream clients (app-mftodo, app-mfgit, `@moduleforge/tags-gui`) and state that adoption in the apps (pin bumps, regeneration) belongs to the composing apps.
5. No links from these docs into `plan/`.

## Validation

- Every entry under the wire-risk note's "Recorded by tasks" maps to a doc change; list the mapping in the report.
- `grep -rn '](.*plan/' README.md AGENTS.md api/README.md docs/` shows no links into `plan/` except allowed durable artifacts.
- `python3 -c 'import yaml; yaml.safe_load(open("api/openapi.fragment.yaml"))'` succeeds.

## Metadata

architectural_impact: false

## Assumptions

- Phase 09 has landed.

## Status

Outcome: succeeded (2026-10-14). Docs only: `api/openapi.fragment.yaml`, `AGENTS.md`, `README.md`, `api/README.md`.

Mapping of wire-risk "Recorded by tasks" entries: 7.3 and 7.5 (RequireAuth, sudo-only 403) to the fragment description, `POST`/`GET /tags` and `GET /entities/{uuid}/tags` descriptions, AGENTS.md Router mounting, README, api/README; in-process ErrForbidden change to AGENTS.md downstream-clients section; 8.1 decode/cap and `invalid_uuid` details to the fragment (info and body routes) and AGENTS.md; 8.1 stateless-before-authorize to AGENTS.md Conventions and fragment; NUL 400 (8.1 and 9.1) to the fragment and AGENTS.md; 9.1 one_of_domain stays 409 and `GET /tags/{uuid}` post-authorization 404 to the fragment and AGENTS.md; p3Gt documented as a known open security issue in the fragment, AGENTS.md and README.

Validation: YAML parses; no `plan/` links found in README.md, AGENTS.md, api/README.md, docs/.

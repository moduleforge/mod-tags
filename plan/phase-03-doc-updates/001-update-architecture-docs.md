# Update Architecture Docs

## Purpose and scope

Reconcile mod-tags' own documentation with the public API-response behaviour changed by this
session's Go work: the new **nested** error envelope, the canonical error-code vocabulary
(`unauthenticated`/`invalid_input` replacing `unauthorized`/`bad_request`), and the
existence-masking behaviour (genuine entity misses now return `403 forbidden` instead of
`404 not_found`). Runs last, after Phases 01 and 02 have landed. Invoke the
[`update-architecture-docs`](../../../../../sdlcforge/flow/plugins/flow/task-procedures/update-architecture-docs/SKILL.md)
task-procedure (`plugins/flow/task-procedures/update-architecture-docs/SKILL.md`).

## Requirements

Review and, where stale, update mod-tags' documentation to match the implemented behaviour.

**Implementation task docs that surfaced these architectural implications** (both flagged
`architectural_impact: true`):

- `plan/phase-01-go-apiresp-migration/001-response-layer-migration.md` — nested error envelope +
  canonical error codes (public API contract change).
- `plan/phase-01-go-apiresp-migration/002-entityresolver-masking-adoption.md` — 404→403 masking
  status-code behaviour change (public API behaviour change).

**Documentation files to review** (mod-tags has **no** `docs/architecture.md` and **no**
`docs/*-spec.md` — confirmed via glob; the design/spec home for the cross-cutting contract is the
out-of-scope submodule doc `docs/mf-standards/architecture/api-response-design.md`, which is *not*
edited here). Review the module's actual docs:

- **`next-steps.md`** — the most affected. Its "Pending manual verification" curl smokes and the
  "carry-forward" integration scenario assert `GET /v1/tags/{uuid} → 404` for a missing/absent tag
  (e.g. line ~29) and describe `{tags: […]}` list behaviour; the `404` expectations for genuine
  misses are now `403` under masking. Correct these. Also note the "List envelope asymmetry" item
  remains accurate (that deviation was intentionally left in place).
- **`AGENTS.md`** — the route table and "Conventions" section. Add/adjust any statement of the
  HTTP error-response shape and status semantics so it reflects the standardized nested envelope,
  canonical codes, and masking-by-default (403 on genuine miss). Do not duplicate the design doc;
  link to `docs/mf-standards/architecture/api-response-design.md` as the canonical contract.
- **`README.md`** — high-level; update only if it states an error/response shape that is now stale
  (likely no change needed).
- **`docs/decisions/tags-limited-immutability.md`** — review for any now-stale error/status claim
  (likely no change; confirm).

Keep edits minimal and accurate: correct stale statements, add a canonical-contract pointer where
the error behaviour is described, and do not restate the design doc's content.

role_doc: plugins/flow/references/roles/architect-backend.md

## Validation

- `next-steps.md` no longer asserts `404` for a genuine tag/entity miss where masking now returns
  `403`; its list-envelope note remains consistent with the (unchanged) `{tags: […]}` behaviour.
- `AGENTS.md`'s error/response description (if any) matches the implemented nested envelope +
  canonical codes + masking behaviour, with a link to the canonical design doc.
- `grep -rn "not_found\|404\|bad_request\|unauthorized" *.md docs/*.md docs/decisions/*.md` surfaces
  no remaining doc statement that contradicts the implemented behaviour (excluding intentional
  router-level-404 or historical references).
- A reviewer confirms each named file was reviewed and updated where needed, and that no mod-tags
  doc now contradicts `docs/mf-standards/architecture/api-response-design.md`.

## Assumptions

- Phases 01 and 02 are complete and merged, so the implemented behaviour is final and reviewable.
- The submodule design doc `docs/mf-standards/architecture/api-response-design.md` is the source of
  truth and is **not** edited by this task.

## References

- `plugins/flow/task-procedures/update-architecture-docs/SKILL.md` — the task procedure to follow.
- [`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
  — the canonical contract mod-tags' docs must not contradict.
- The two Phase 01 task docs named above.

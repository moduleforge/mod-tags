# mod-tags — next steps

All 6 planned phases (bootstrap → model → API → GUI → wire into mod-users → verify) have been implemented. Items below are pending manual verification or deferred work that surfaced during implementation. Original phase reports were in `plan/` (now removed); this file is the forward-looking residue.

## Pending manual verification (needs live stack / DB)

- **`make dev.start` smoke.** Bring up mod-users's composed stack, authenticate, then round-trip via curl:
  - `POST /v1/tags` `{subject, purpose: "rating", value: "5", color: "#FF0000FF"}` → 201.
  - `GET /v1/tags/{uuid}` → 200, fields match.
  - `GET /v1/entities/{subject_uuid}/tags` → 200, `{tags: [...]}` contains the tag.
  - `PUT /v1/tags/{uuid}` `{"color": null}` → 200, color cleared.
  - `PUT /v1/tags/{uuid}` `{"purpose": "x"}` → 400 (DisallowUnknownFields).
  - `DELETE /v1/tags/{uuid}` → 204.
- **`atlas migrate status`** against a live DB should show `0000–0011` (core) → `0100–0109` (users) → `0200–0201` (tags), no gaps.
- **Audit log** — `SELECT * FROM audit_log WHERE resource = 'tag' ORDER BY id DESC LIMIT 10` should show create/update/delete entries with the acting principal's entity id.
- **UI smoke.** Drop `<TagEditor subject={user.uuid} />` into an admin page and verify add / remove / color-edit / clear flows end-to-end.

## Known carry-forward items (non-blocking)

- **No DB-backed integration test in mod-users.** Task 5.5 (HTTP-level integration test that creates / reads / updates / deletes a tag through mod-users's composed server) was skipped — mod-users has no testcontainer harness, and setting one up just for tags would be ~150–200 lines and prejudge broader test-infra decisions. When mod-users grows a general harness, add the tags test at that time. The scenario to add:
  1. Register + authenticate a non-admin user.
  2. `POST /v1/tags` → 201.
  3. `GET /v1/tags/{uuid}` → 200.
  4. `GET /v1/entities/{subject_uuid}/tags` → tag in list.
  5. `PUT /v1/tags/{uuid}` `{"color": "#00FF00FF"}` → 200.
  6. `PUT /v1/tags/{uuid}` `{"color": null}` → 200, color cleared.
  7. `PUT /v1/tags/{uuid}` `{"purpose": "x"}` → 400.
  8. `DELETE /v1/tags/{uuid}` → 204.
  9. `GET /v1/tags/{uuid}` → 404.
  10. Audit log shows the create/update/delete chain.
- **Service coverage is 62%** (below the 70% target) because Create/Delete tx paths require real tx behavior. Handler tests exercise these paths end-to-end via a fake service, so behavior is covered; coverage metric isn't.
- **List envelope asymmetry.** `GET /tags` returns a bare array; `GET /entities/{uuid}/tags` returns `{tags: [...]}`. Client handles both; worth standardizing in a future pass.
- **N+1 on owner/subject UUID resolution in `Search` and `ListBySubject` hydration.** Phase 1 access-fn rewrite returned the tag's own UUID via JOIN, but owner_id and subject_id are still resolved per-row via `GetEntityByID` in the service layer (`tag.go` ~line 350, ~line 354). For paged niche-app scale this is acceptable; if it becomes hot, batch via `GetEntitiesByIDs(IN ...)` or extend the SQL JOIN.
- **`display.Registry.Render` unused at runtime.** `coreservice.RegisterBuiltins` is now wired in mod-users main.go (first consumer), but no production code path currently calls `Render`. Becomes load-bearing if/when a UI surface needs server-rendered entity display names.

## Component workbench (Ladle)

See `stories-next.md` at this module's root for deferred Ladle / Storybook follow-ups (story coverage gaps including `TagChip` truncation + `TagEditor` multi-purpose select, mock-vs-real client decorator, Storybook migration path, visual regression).

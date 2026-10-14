# HTTP Error Precedence Conformance Decision Record

## Purpose and scope

This decision record documents how mod-tags applies the shared HTTP error-precedence
rule ([`docs/mf-standards/architecture/http-error-precedence.md`](../mf-standards/architecture/http-error-precedence.md))
to its routes: where authentication is enforced, which checks run before
authorization, how a sudo-only identity is answered, and why the one-of-domain
conflict stays `409`. It records only the mod-tags choices; the rule itself is not
restated here. The wire-level summary lives in [`AGENTS.md`](../../AGENTS.md#router-mounting)
and the per-operation contract in
[`api/openapi.fragment.yaml`](../../api/openapi.fragment.yaml).

## Context

The rule orders failures `401`, `400`, `403`, `404`, then `409`/`422`. Before this
change, mod-tags authenticated inside each handler, decoded bodies without a size
cap, and validated `POST /tags` and `GET /tags` input only after authorizing, so a
caller without permission could not tell a malformed request from a forbidden one.

## Decision

### Authentication in the router

`RegisterRoutes` mounts `authz.RequireAuth` on an inner `chi` group that holds every
mod-tags route, and `NewRouter` delegates to `RegisterRoutes`, so both entry points
answer an anonymous request with `401` before anything is read. The middleware is on
the inner group, never on the supplied router, so routes a host registers beside
`RegisterRoutes` stay ungated. `GET /tag-templates` sits in the group: it is
authenticated but intentionally not authorized (an open, catalog-only read).

### Stateless shape before authorization

- Body routes decode with `apiresp.DecodeJSON` and the `tags` code prefix: a 1 MiB
  cap and rejection of trailing data, reported as `tags.body_too_large` and
  `tags.invalid_json_body` details.
- Path, query and body UUIDs are parsed with `apiresp.ParseUUIDParam`, which reports
  an `invalid_uuid` detail for a malformed value.
- `TagService.Create` and `TagService.Search` run their stateless validation (required
  fields, length caps, color format, the owner-or-subject filter requirement, and
  `tags.nul_character` for a NUL in `purpose`/`value`) before the `Authorizer` call.
  `TagService.ListBySubject` rejects a NUL purpose filter before resolving the
  subject. Without the NUL check a NUL would pass authorization and surface as a
  `500` from Postgres text.

### Sudo-only identity is `403`

`Create`, `Search` and `ListBySubject` use the regular actor as owner or row scope,
and a sudo identity is never used as an owner. They therefore call
`requireRegularActor`: no identity at all is `401`, and an authenticated sudo-only
identity is `403` (masked `forbidden`). A `403` rather than a `401` or a fallback to
the sudo id was chosen because the caller is authenticated but has no actor to own or
scope tags. This guard runs before stateless validation, so a sudo-only caller gets
`403` even for an invalid request. In-process callers of these service methods see
`ErrForbidden` where they previously saw `ErrUnauthenticated` for a sudo-only context.

### Existing masked misses are the `403` stage

UUID-keyed lookups (`GET`/`PUT`/`PATCH`/`DELETE /tags/{uuid}`,
`GET /entities/{uuid}/tags`) keep their masked `403` for a genuine miss, as do the
subject lookup in `Create` and the in-transaction miss on update and delete. These
are unchanged; they are the rule's `403` stage, not a `404`. `GET /tags/{uuid}`
returns `404` only after authorization, for a resolved entity that has no tag row.

### One-of-domain stays `409`

A one-of-domain duplicate is a uniqueness conflict with an existing tag, which is the
rule's `409` case. It is not a reference to a value outside a registry, which is what
`422` would describe. It is also what `TagEditor` (`gui/src/TagEditor.tsx`) already
handles, so `422` would break the GUI. The trigger raises SQLSTATE `23505` inside the
create transaction, after `Authorize`, and `Create` maps that to `ErrConflict`. Once
the one-of-domain trigger replaced the unique index, it is the only source of
`23505` on `tags`, and because it raises without a constraint name the mapping cannot
be narrowed to a constraint. Tests in `api/service/tag_conflict_test.go` pin the
mapping and that a denied caller never reaches the insert. The trigger's own
record is [`tags-one-of-domain.md`](./tags-one-of-domain.md).

## Consequences

| Aspect | Status after this change |
|---|---|
| Anonymous request, any route | `401` before the request is read, on both `RegisterRoutes` and `NewRouter`. |
| Invalid request from an unauthorized caller | `400`, not `403` (except a sudo-only identity on `POST /tags`, `GET /tags`, `GET /entities/{uuid}/tags`, which is `403` first). |
| Sudo-only identity | `403` on `POST /tags`, `GET /tags`, `GET /entities/{uuid}/tags`. |
| One-of-domain duplicate | `409` `conflict`, only after authorization. |
| `GET /tag-templates` | Authenticated, not authorized. |
| Open issue `p3Gt` | `POST /tags` authorizes the caller against the caller's own entity, never against the subject. An existing subject answers `201` (or `409`) while a missing subject answers the masked `403`, so an authenticated caller allowed to create tags can probe whether an entity UUID exists and can tag an entity it cannot access. Not fixed by this change. |
| Composing apps | app-mftodo, app-mfgit and `@moduleforge/tags-gui` observe the wire changes; adopting them (pin bumps, client regeneration) belongs to those apps. |

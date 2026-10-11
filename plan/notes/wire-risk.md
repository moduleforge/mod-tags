# Wire Risk

> **Ephemeral plan document.** This file is ephemeral and will be removed after the implementation is complete. Project documents must be updated or created to capture the context and information contained herein. Link only from other plan documents; do not link to this document from non-plan documents.

## Purpose and scope

The mod-tags running list of wire-visible changes (common rule 4). Each implementing task appends or corrects entries for what it actually changed, under [Recorded by tasks](#recorded-by-tasks). Task 10.1 copies the final list into project docs and names the downstream clients. The planned list below is the planner's expectation, not a record. Decision ids (D1-D11) refer to the [handler inventory](./handler-inventory.md#planning-decisions).

Downstream clients: app-mftodo and app-mfgit (both compose `RegisterRoutes`; local builds track `main` through relative replaces; lockfile pins are bumped by the mftodo-mfgit plan-group); `@moduleforge/tags-gui` (`TagEditor`, `mockClient.ts`) as consumed by the apps' GUIs.

## Planned changes

1. **401 before 400 for unauthenticated callers.** `RequireAuth` runs inside `RegisterRoutes`, and `NewRouter` now delegates to it (D1). Inside the composing apps the app's own `requireAuth` already rejected these first.
2. **Sudo-only identities are authenticated.** Every inline check was sudo-unaware (401). A sudo-only identity now passes; `POST /tags`, `GET /tags` and `GET /entities/{uuid}/tags` answer it with 403 (D3); `GET /tag-templates` serves it; the UUID routes proceed to normal authorization.
3. **Body cap and decode details (D5).** Body routes newly cap bodies at 1 MiB (was unbounded). Trailing data after the JSON value is newly rejected. Decode failures newly carry a `details` entry (`tags.invalid_json_body`, `tags.body_too_large`, with the field name for a wrong-typed member). The top-level code and message are unchanged.
4. **UUID details (D6).** Malformed path, query and body UUIDs newly carry an `invalid_uuid` detail naming the field (`uuid`, `owner`, `subject`, `scope`). The top-level code is unchanged.
5. **Stateless validation precedes authorization (D7).** A caller without permission who sends an invalid request gets 400 instead of 403 on `POST /tags` (purpose/value/color) and `GET /tags` (neither `owner` nor `subject`).
6. **Post-authorization classification (D8; planned, decided by task 9.1).** `one_of_domain` stays 409; no other status change is planned.

In-process callers:

7. `Create`, `Search` and `ListBySubject` return `ErrForbidden` instead of `ErrUnauthenticated` for a sudo-only context (D3), and `Create`/`Search` return validation errors before authorization errors (D7).

Not changing: existence masking (already 403, D4); success bodies; the 409 for one-of-domain conflicts.

## Recorded by tasks

Tasks append dated entries here: route, old behavior, new behavior, client that may notice.

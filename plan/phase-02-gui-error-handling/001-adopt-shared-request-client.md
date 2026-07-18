# Adopt Shared Request Client

## Purpose and scope

Replace mod-tags' hand-rolled `fetch`/error-parsing in `gui/src/lib/api.ts` with
`@moduleforge/core-gui`'s promoted, shared typed `request()` wrapper and `ApiRequestError`/wire
types, so the tags GUI client parses the **nested** error envelope
(`{"error":{"code","message","details?"}}`) and throws typed `ApiRequestError`s instead of bare
`Error(message)`. This is the client-layer half of the GUI migration; the component-layer half is
task 002. TypeScript-only, under `gui/`.

## Requirements

1. **Depend on `@moduleforge/core-gui` for real.** It is currently an *optional* peer dependency
   (`gui/package.json` → `peerDependenciesMeta["@moduleforge/core-gui"].optional: true`). Promote it
   to a required dependency of the tags GUI (remove the `optional` flag and/or add it to
   `peerDependencies`/`dependencies` per the ecosystem convention used by the other GUI packages).
2. **Rewrite `gui/src/lib/api.ts`'s transport/error layer.** Remove the bespoke `handleResponse<T>`
   helper, the local `ErrorResponse` interface, and the `throw new Error(message)` logic. Route
   every request (`listBySubject`, `create`, `updateColor`, `updateValue`, `remove`) through
   `@moduleforge/core-gui`'s `request()` wrapper, which owns method/headers/body, JSON parsing, the
   204-no-body case, the nested-envelope parse, the synthesized `network_error`/status-0 transport
   failure, and the `401`-redirect special case. Keep the client's **public surface** unchanged:
   `createTagsClient(opts)` returning the same five methods with the same signatures, and the
   exported `Tag` / `TagsClientOptions` types (so `TagList`/`TagEditor` imports and consumers are
   unaffected at this step).
3. **Preserve request specifics** that `request()` does not own: the `baseUrl` composition, the
   `headers()` injection hook (Authorization etc.), the `?purpose=` single-purpose query
   optimization and the multi-purpose client-side filter in `listBySubject`, and the JSON bodies for
   create/update. Reconcile these against `request()`'s actual option shape (base URL, header
   provider, `skipAuthRedirect`, etc.) as exported by Wave 0.
4. **Keep the list-response extraction.** `listBySubject` currently reads `body.tags` from the
   server's `{"tags": Tag[]}` response. The server list-envelope shape is **not** changing in this
   plan, so keep extracting `.tags` from the parsed body (e.g. `request<{tags: Tag[]}>(...)` then
   `.tags ?? []`). Do not assume a bare array here.
5. **Types.** Re-export or re-use `@moduleforge/core-gui`'s wire types (`FieldError`, `ApiError`,
   `ApiErrorResponse`) and `ApiRequestError` rather than redefining them locally. Keep the local
   `Tag` interface (it is the tags resource shape, not an error type).

## Validation

- `cd gui && bun run typecheck` exits 0.
- `cd gui && bun run build` succeeds.
- `grep -n "throw new Error\|handleResponse\|interface ErrorResponse" gui/src/lib/api.ts` returns
  nothing — the bespoke transport/error layer is gone.
- `grep -n "core-gui" gui/package.json` shows `@moduleforge/core-gui` as a non-optional dependency.
- `createTagsClient`'s exported signature and the `Tag`/`TagsClientOptions` exports in
  `gui/src/index.ts` are unchanged (consumers and `TagList`/`TagEditor` still compile).
- Errors thrown by the client are `ApiRequestError` instances carrying `code`/`status`/`details`.

## Assumptions

- **Wave 0 is merged** and `@moduleforge/core-gui` exports the shared `request()` wrapper, the wire
  types, and `ApiRequestError`. If not, halt and report.
- The exact `request()` option/return shape and export names are taken from the design-doc contract
  (§"GUI-facing error-data contract", §"Client contract (`ApiRequestError`)"); reconcile against
  Wave 0's actual exports before coding.
- The server response shapes are unchanged by this plan: single resources are bare objects; the
  `GET /entities/{uuid}/tags` list is `{"tags": […]}`; errors are the nested envelope (emitted by
  Phase 01, though this task does not depend on Phase 01 landing — the GUI is validated against the
  mock client / Ladle, not a live server).

## References

- [`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
  §"GUI-facing error-data contract" (wire types, `ApiRequestError`, `network_error`/401 special
  cases).
- Current code: `gui/src/lib/api.ts` (transport/error layer to replace), `gui/package.json`
  (dependency promotion), `gui/src/index.ts` (public exports to preserve).

## Checkpoint hints

- After promoting the `@moduleforge/core-gui` dependency and confirming `bun run typecheck` resolves
  the import.
- After rewriting `api.ts` onto `request()` and confirming the public surface is unchanged.

## Status

**Outcome: succeeded** (2026-07-18)

- **Dependency promotion.** `gui/package.json` → removed `peerDependenciesMeta["@moduleforge/core-gui"]`
  entirely (was `{ optional: true }`); `@moduleforge/core-gui` remains in `peerDependencies` as `"*"`,
  matching the convention used by `mod-users/gui`, `mod-tasks/gui`, and `mod-contacts/gui` (peer, not a
  bundled runtime `dependencies` entry). Because Wave 0 (`@moduleforge/core-gui`) is not published to a
  real npm registry, Bun's default auto-install-peers behavior tried to fetch the now-non-optional peer
  from `registry.npmjs.org` and 404'd. Added an `"overrides": { "@moduleforge/core-gui":
  "file:.yalc/@moduleforge/core-gui" }` entry (the minimal, standard npm/bun mechanism for pinning a
  peer's local resolution) so `bun install` resolves it from the local yalc store instead of the
  registry; `yalc add @moduleforge/core-gui` (from the pre-populated global yalc store — Wave 0's `dist/`
  was already built and yalc-published) populated `.yalc/@moduleforge/core-gui` and
  `node_modules/@moduleforge/core-gui` (both gitignored, consistent with the rest of the ecosystem).
  `bun install` re-run and verified green; `bun.lock`'s stale `optionalPeers` entry is gone.
- **`gui/src/lib/api.ts` rewrite.** Removed `handleResponse<T>`, the local `ErrorResponse` interface,
  the `throw new Error(...)` fallback, and the `fetchImpl` escape hatch (dropped deliberately: Wave 0's
  `request()` always calls the global `fetch` directly and offers no fetch-injection seam, and `fetchImpl`
  had zero call sites anywhere in `gui/src` — grep-confirmed before removal). All five methods
  (`listBySubject`, `create`, `updateColor`, `updateValue`, `remove`) now route through
  `@moduleforge/core-gui`'s `request<T>(url, options)`. Preserved: `baseUrl` composition (full URL built
  locally, since `request()` takes `string | URL` with no base-URL concept of its own), the `headers()`
  injection hook (still computed via `buildHeaders()` and passed as `options.headers`, since `request()`
  has no per-call header-provider parameter — auth-token attachment is instead handled internally by
  `request()`'s own module-level `authHandler` seam), the `?purpose=` single-purpose optimization plus
  client-side multi-purpose filter, and the JSON `body: JSON.stringify(...)` payloads for create/update.
  `listBySubject` still extracts `.tags ?? []` from `request<{tags: Tag[]}>(...)`. `createTagsClient`'s
  signature and the five method signatures are byte-identical to before.
- **Types.** Re-exported (not redefined) `@moduleforge/core-gui`'s wire types and error class:
  `ApiError`, `ApiErrorResponse`, `FieldErrorData` (type-only) and `ApiRequestError` (value/class).
  Reconciliation note: the design-doc contract referenced in this task's `## Requirements` names the
  field-error type `FieldError`; Wave 0's actual export (confirmed by reading
  `mod-core/gui/src/lib/api-types.ts` directly) is `FieldErrorData` — used the real exported name.
  `gui/src/index.ts` was left untouched per this task's own Validation bullet, so these re-exports are
  reachable only via `gui/src/lib/api.ts` for now (not yet surfaced at the package root); task 002
  (component-layer half) is expected to decide whether/how to surface them from `index.ts`.

### Validation results

- `cd gui && bun run typecheck` — **passed**, clean exit 0, no diagnostics.
- `cd gui && bun run build` — **passed**, exit 0. `tsup`'s DTS step emits a benign warning
  ("ApiError", "ApiErrorResponse", "FieldErrorData" and "ApiRequestError" ... never used in
  "src/lib/api.ts") because `gui/src/index.ts` doesn't currently forward these re-exports into the
  package's public entry graph (see Types note above); the build still succeeds and `dist/index.d.ts`
  correctly retains `Tag`, `TagsClientOptions`, and `createTagsClient`.
- `grep -n "throw new Error\|handleResponse\|interface ErrorResponse" gui/src/lib/api.ts` — **passed**,
  no matches.
- `grep -n "core-gui" gui/package.json` — **passed**, shows `@moduleforge/core-gui` in
  `peerDependencies` and `overrides`, no `optional` occurrences anywhere in the file (`grep -c
  "optional" gui/package.json` → 0).
- `createTagsClient`'s exported signature and the `Tag`/`TagsClientOptions` exports in `gui/src/index.ts`
  — **passed**, `gui/src/index.ts` has zero diff (`git diff` empty); `bun run typecheck`/`build` succeeding
  confirms `TagList.tsx`/`TagEditor.tsx` still compile against the unchanged types.
- "Errors thrown by the client are `ApiRequestError` instances carrying `code`/`status`/`details`" —
  **passed by construction**, not an executable check (no test runner is configured for `gui/` in this
  package — no `test` script, no existing test files under `gui/src`, consistent with the project's
  current state). Every one of the five client methods now throws exclusively via
  `@moduleforge/core-gui`'s `request()`, whose documented contract (verified by reading
  `mod-core/gui/src/lib/api-client.ts`) throws `ApiRequestError(code, message, status, details)` on every
  non-2xx response and on transport failure (`network_error`, status `0`); no other throw site remains
  in `api.ts`.

### Assumptions applied

- Wave 0 merged and `@moduleforge/core-gui` exports `request()`, the wire types, and `ApiRequestError` —
  confirmed directly against `mod-core/gui/src/lib/api-client.ts` / `api-types.ts`.
- Reconciled the design-doc-derived contract against Wave 0's actual exports per the task's own
  Assumptions: `request()` has no base-URL concept, no per-call header-provider parameter, and no
  fetch-injection seam; `FieldErrorData` is the real export name (not `FieldError`).
- Server response shapes unchanged (bare single-resource objects, `{"tags": [...]}` list envelope,
  nested error envelope) — relied on for the `.tags ?? []` extraction and the removal of any bespoke
  error-body parsing.

### Decisions made

- Removed `TagsClientOptions.fetchImpl` (no call sites anywhere in `gui/src`; `request()` provides no
  seam to honor it) rather than keeping an inert field — see Types/rewrite note above.
- Kept `@moduleforge/core-gui` in `peerDependencies` only (not `dependencies`), matching every sibling
  GUI package's convention, and added a package.json `"overrides"` entry (rather than a `dependencies`
  or `devDependencies` file: entry, both of which still triggered Bun's registry fetch attempt in
  testing) to make `bun install` resolve the peer locally via yalc.

### Affected files

- `gui/src/lib/api.ts`
- `gui/package.json`
- `gui/bun.lock`
- `plan/phase-02-gui-error-handling/001-adopt-shared-request-client.md` (this file)

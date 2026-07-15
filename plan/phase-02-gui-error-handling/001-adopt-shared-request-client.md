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

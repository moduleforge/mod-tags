# Migrate Error Widgets

## Purpose and scope

Replace mod-tags' most-duplicated ad-hoc error handling — the ~8 near-identical
`err instanceof Error ? err.message : '<fallback>'` catch blocks, local `submitError`/`fetchError`/
`errorMessage` string state, and inline `<span role="alert">` markup across `TagEditor.tsx` and
`TagList.tsx` — with `@moduleforge/core-gui`'s `useApiError` hook and `<FieldError>`/`<ErrorBanner>`
widgets (routing toast-worthy failures to the Toast provider). Consumes the typed
`ApiRequestError`s now thrown by the client (task 001). TypeScript/React-only, under `gui/`.
Sequenced **after** task 001.

## Requirements

1. **`TagEditor.tsx` — migrate all six catch blocks + inline error UI.** Current ad-hoc sites:
   - `fetchTags` catch (~lines 59-64) and the mount-effect `.catch` (~lines 79-84) → load errors.
   - `handleRemove` catch (~lines 106-110), `handleColorChange` catch (~lines 119-122),
     `handleValueChange` catch (~lines 131-134) → mutation errors currently funneled into
     `fetchError`.
   - `handleAddSubmit` catch (~lines 173-176) → `submitError`.
   - Local state `submitError` (~line 47) and `fetchError` (~line 35); inline renders at ~line
     195-197 (load error) and ~line 324-328 (submit error).
   Replace the hand-rolled `err instanceof Error ? err.message : …` extraction with `useApiError`
   applied to the caught `ApiRequestError`, rendering banner-level errors via `<ErrorBanner>` and
   any field-bound `details` via `<FieldError>` beside the relevant input (the add-form
   purpose/value inputs are the natural binding targets for `invalid_input` field details, e.g. a
   `409 conflict` "tag already exists" surfaces as a banner). Dispatch toast-worthy classes
   (`network_error`, `internal_error`, optimistic-update rollbacks) to the Toast provider.
2. **`TagList.tsx` — migrate its instance.** The `.catch` at ~lines 42-47 extracting into
   `errorMessage` (~line 25) and the inline `<span role="alert">` render at ~lines 66-72 → replace
   with `useApiError` + `<ErrorBanner>` for the load-error path.
3. **Preserve UX behaviour** — loading skeletons, the `LoadState` machine, empty-state rendering,
   the add-form's local *validation* messages ("Purpose is required." / "Value is required." set at
   ~lines 148/152 before the request) may remain as local pre-submit validation, or be routed
   through `<FieldError>` for consistency; keep them user-visible either way. Do not regress
   accessibility (the `role="alert"` semantics should be provided by the shared widgets).
4. **Update the Toast provider wiring.** If `useApiError`'s toast dispatch requires a
   `ToastProvider` context, ensure the components document/assume it is mounted by the host app, and
   that the Ladle stories wrap the components in the provider so toast-worthy paths render in the
   workbench.
5. **Update mock client + stories to exercise the new surfaces.** `gui/src/lib/mockClient.ts`
   currently throws bare `new Error('Mock failure: …')` for its `failOn` cases; update it to throw
   `ApiRequestError`s (with representative `code`/`status`/`details`, e.g. a `403 forbidden`, a
   `409 conflict` with a field detail, a `network_error`) so `TagEditor.stories.tsx` /
   `TagList.stories.tsx` demonstrate `<FieldError>`/`<ErrorBanner>`/toast routing. Add or adjust
   stories covering the field, banner, and toast surfaces.
6. **Exports.** Keep `TagEditor`/`TagList`/`TagChip` and their prop types exported unchanged from
   `gui/src/index.ts`.

## Validation

- `cd gui && bun run typecheck` exits 0; `cd gui && bun run build` succeeds.
- `grep -rn "instanceof Error" gui/src` returns nothing (all ad-hoc extraction removed).
- `grep -rn 'role="alert"' gui/src` returns nothing outside the shared widgets (inline alert markup
  removed from `TagEditor.tsx`/`TagList.tsx`).
- `TagEditor.tsx` and `TagList.tsx` import `useApiError` and `<FieldError>`/`<ErrorBanner>` from
  `@moduleforge/core-gui`; no local `submitError`/`fetchError`/`errorMessage` string-plumbing of
  error *messages* remains (local pre-submit validation state, if retained, is acceptable).
- `gui/src/lib/mockClient.ts` throws `ApiRequestError` for its failure cases.
- Ladle workbench (`cd gui && bun run dev`, or `make preview`) renders the field, banner, and
  toast-worthy error surfaces from the updated stories without runtime error.

## Assumptions

- **Wave 0 is merged** and `@moduleforge/core-gui` exports `useApiError`, `<FieldError>`,
  `<ErrorBanner>`, and the Toast provider/`useToast`. Task 001 is complete, so the client throws
  `ApiRequestError`. If either precondition is unmet, halt and report.
- Widget names, the `useApiError` return shape (`{ fieldErrors, bannerError }` + toast dispatch),
  and the Toast provider API are taken from the design-doc contract; reconcile against Wave 0's
  actual exports before coding.
- The backend does not emit structured `details` for tags today, so field-level binding is
  exercised primarily via the mock client / stories; real banner-level surfacing (`forbidden`,
  `conflict`, `network_error`) is the main live path.

## References

- [`docs/mf-standards/architecture/api-response-design.md`](../../docs/mf-standards/architecture/api-response-design.md)
  §"Surface classification" (field vs banner vs toast routing) and §"Widget set implied".
- Current code: `gui/src/TagEditor.tsx`, `gui/src/TagList.tsx`, `gui/src/lib/mockClient.ts`,
  `gui/src/TagEditor.stories.tsx`, `gui/src/TagList.stories.tsx`.

## Checkpoint hints

- After migrating `TagList.tsx` (the simpler single-instance case).
- After migrating `TagEditor.tsx`'s six sites + inline renders.
- After updating `mockClient.ts` and the stories to exercise field/banner/toast surfaces.

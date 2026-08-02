# Tag Editor One Of Domain Filtering

## Purpose and scope

Add client-side `one_of_domain` awareness to `TagEditor`: exclude already-occupied
one-of-domain purposes from the `<select>` picker, and block submission client-side
(any mode) of a purpose+value pair that would violate exclusivity — fixing, as part of
this same change, the pre-existing bug where no client-side purpose-conflict check
exists at all today (see `plan/overview.md`, Design decision 5, for the full
root-cause writeup and the "combo-search picker" scope clarification — read that
section before starting; it resolves an ambiguity between the request's wording and
the actual current `TagEditor` code that would otherwise cause scope confusion).
**Depends on** Phase 2 being complete (needs the real `oneOfDomain` wire field).

## Requirements

### 1. `gui/src/lib/api.ts` — `Tag` gains `oneOfDomain`

Add `oneOfDomain: boolean;` to the `Tag` interface (required, not optional — the
server always computes and returns it per Phase 2).

### 2. `gui/src/TagEditor.tsx` — occupied-purpose computation

Add a derived value (e.g. `useMemo` or a plain computation each render — match this
file's existing style, which does not currently use `useMemo` elsewhere, so a plain
computation is likely more consistent):

```ts
const occupiedPurposes = new Set(tags.filter((t) => t.oneOfDomain).map((t) => t.purpose));
```

Compute this from the component's existing `tags` state (already fetched via
`fetchTags`/the mount `useEffect`) — no new network call is needed (see
`plan/overview.md`'s Design decision 4 for why the API design deliberately makes this
possible without a second endpoint).

### 3. `<select>` (multi-purpose) mode — filter options

In the `isSelectPurpose` branch's `<select>` (currently maps `purposes!.map((p) =>
<option key={p} value={p}>{p}</option>)`), exclude any `p` in `occupiedPurposes` from
the rendered options. Do not filter out the currently-selected `addPurpose` value even
if it happens to be in `occupiedPurposes` (this should not normally happen since it was
excluded from the options in the first place, but avoid introducing a state where a
previously-valid selection silently disappears from a controlled `<select>` — filter
the `.map()` source, not the selected value).

### 4. Pre-submit validation — all modes

In `handleAddSubmit`, after the existing empty-purpose/empty-value checks (which use
`setPreSubmitFieldError`) and before the `client.create(...)` call, add:

```ts
if (occupiedPurposes.has(purposeToSubmit)) {
  setPreSubmitFieldError({
    field: 'purpose',
    code: 'client.one_of_domain_conflict',
    message: `Only one "${purposeToSubmit}" tag is allowed on this item.`,
  });
  return;
}
```

Place this check so it applies uniformly regardless of which purpose-input mode is
active (free-form, fixed, or `<select>`) — `purposeToSubmit` is already computed
upstream of the existing empty-checks for exactly this reason; reuse it, don't
recompute. This is the fix for the reported bug: today, nothing between
`purposeToSubmit` being computed and `client.create` being called validates it against
the existing tag list at all.

In fixed-purpose mode (`isFixedPurpose`), this means the add-form now correctly
refuses a second submission for a purpose that already has a one-of-domain tag,
surfaced as a field error under the (currently static, non-input) purpose display —
`purposeFieldError`'s existing rendering already falls through to
`preSubmitFieldError` when `preSubmitFieldError?.field === 'purpose'` regardless of
which purpose-UI branch is active, so no additional rendering change should be needed
for this to display; verify this is actually the case when testing (there is no
`<FieldError>` slot in the fixed-purpose branch today — confirm whether the error is
visible anywhere in that mode, and if not, add a `<FieldError>` render next to the
fixed-purpose `<span>`, matching the free-form branch's slot).

### 5. `gui/src/lib/mockClient.ts` — seed `oneOfDomain`

`createMockTagsClient`'s `create` method currently builds a `Tag` literal without
`oneOfDomain`. Since the interface field is now required, every `Tag` constructed here
(and any seeded via `opts.initial`) needs a value. For the mock's `create` return
value, default to `false` (the mock has no policy-table concept to consult — this is
acceptable for a story/dev mock; do not build a parallel policy-simulation layer here,
that is out of scope). Callers that want to demonstrate the exclusion behavior in a
story (Requirement 6) pass `oneOfDomain: true` directly in their seeded `initial` tags.

### 6. `gui/src/TagEditor.stories.tsx` — new story

Add a story demonstrating the exclusion behavior, following the existing
`SelectPurpose` story's shape (multi-purpose `<select>` mode, several seeded tags):
seed one tag with `oneOfDomain: true` for a purpose also present in the `purposes`
array, and confirm (via the story, manually reviewed in Ladle) that purpose no longer
appears as a `<select>` option. Name it something like `SelectPurposeOneOfDomain`. Use
the existing `tag()` helper in that file, extended to accept `oneOfDomain` (it
currently spreads `Partial<Tag> & Pick<Tag, 'uuid' | 'purpose' | 'value'>` — passing
`oneOfDomain: true` through the `Partial<Tag>` half already works without changes to
the helper itself, since `oneOfDomain` is a `Tag` field).

## Validation

- `cd gui && bun run typecheck` passes (`tsc --noEmit`). If `bun install` fails due to
  the pre-existing, separately-tracked `core-gui` hard-dependency issue (followup
  `QDH5` — `mod-tags/gui/package.json` declares `@moduleforge/core-gui` as a hard dep
  rather than the optional-peer pattern its sibling packages use), fall back to
  static/manual verification (read-through + `tsc --noEmit` if dependencies are
  already present from a prior install) and note the blocker in your task report
  rather than silently skipping validation — do not attempt to fix `QDH5` as part of
  this task; it is out of scope here.
- `grep -n "oneOfDomain" gui/src/lib/api.ts` shows the new `Tag` field.
- `grep -n "occupiedPurposes" gui/src/TagEditor.tsx` shows the computed set, its use in
  the `<select>` options filter, and its use in the pre-submit check.
- Manual/story-driven verification (via `bun run dev` / Ladle, if buildable per the
  `QDH5` caveat above): the new `SelectPurposeOneOfDomain` story shows the
  already-occupied one-of-domain purpose excluded from the `<select>`, and attempting
  to submit that purpose via a fixed-purpose or free-form story configured the same
  way is blocked client-side with a visible field error (no network call fires — you
  can confirm this by checking the mock's `create` is not invoked, e.g. via a
  `console.log`/breakpoint during manual review, or by reasoning through the code path
  since `return` precedes `client.create(...)`).
- `TagList.tsx` is unaffected (read-only; no add-form) — no changes expected there;
  confirm with `git diff --stat gui/src/TagList.tsx` showing no changes.

## References

- `plan/overview.md` — Design decision 5 (full root-cause analysis and the
  "combo-search picker" scope-interpretation note — read first).
- `gui/src/TagEditor.tsx` — `handleAddSubmit`, `purposeToSubmit`, `isSelectPurpose`,
  `isFixedPurpose`, `preSubmitFieldError`/`purposeFieldError` — the existing pre-submit
  validation pattern this task extends.
- `gui/src/lib/api.ts` — `Tag` interface.
- `gui/src/lib/mockClient.ts` — `createMockTagsClient`.
- `gui/src/TagEditor.stories.tsx` — `SelectPurpose` story and the `tag()` helper, as
  precedent for the new story.
- `stories-next.md` — "TagEditor — multi-purpose select variant with 3+ purposes" was
  already a noted story-coverage gap; this task's new story materially closes it for
  the one-of-domain-aware case (not the whole gap — leave `stories-next.md` itself
  unedited; Phase 4 owns doc updates).

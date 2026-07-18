# @moduleforge/tags-gui

React component library for tag management UIs. Will ship `<TagChip>`, `<TagList>`, and `<TagEditor>` presentational components built on top of `@moduleforge/core-gui` primitives. This package is scaffolded in Phase 1; components land in Phase 4.

## Build

```bash
npm run build
```

Outputs `dist/index.js` (CJS), `dist/index.mjs` (ESM), and `dist/index.d.ts` (types) via tsup.

## Requirements

`<TagEditor>` and `<TagList>` both call `@moduleforge/core-gui`'s `useApiError`
hook unconditionally on every render (not just when an error occurs), and
`useApiError` in turn calls `core-gui`'s `useToast()` unconditionally — which
throws if there is no `<ToastProvider>` ancestor. **Host applications must
mount `@moduleforge/core-gui`'s `<ToastProvider>` above `<TagEditor>` and
`<TagList>` in the component tree**, even if no API errors are ever expected;
otherwise both components will crash on mount.

## Fresh checkout note

`@moduleforge/core-gui` is resolved via yalc in a local development setup. On a fresh checkout, run `make link-tags` from the repo root before `npm install` to publish and link `@moduleforge/core-gui` into this package's `.yalc/` store. Until that step is performed, `@moduleforge/core-gui` may not install correctly without the yalc link.

`@moduleforge/core-gui` is **not published to any public registry** (npm or
otherwise). It is a required (non-optional) peer dependency here, pinned to
`^0.0.0` to reduce (not eliminate) the risk of a dependency-confusion /
scope-squatting attack — the `@moduleforge` npm scope is currently unclaimed
on the public npm registry, and reserving it is tracked separately as an
org-level followup. **Any project that depends on `@moduleforge/tags-gui`
must independently configure its own local resolution for
`@moduleforge/core-gui`** — e.g. via `yalc add @moduleforge/core-gui` plus a
matching `overrides` (npm/bun) or `resolutions` (yarn) entry, the same
mechanism this repo uses (see this package's own `overrides` entry in
`package.json`). A plain `bun install` / `npm install` without that step is
not just "will fail" — it is **unsafe**: if a package were ever published to
the public registry under the unclaimed `@moduleforge` scope, an unpinned or
unresolved install could silently pull in that untrusted package instead of
the intended internal one.

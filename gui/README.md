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

## Consuming this package from an app

An app that composes this module (e.g. `app-mftodo`) wires `@moduleforge/core-gui` and
`@moduleforge/tags-gui` in together through a **bun workspace** it owns — this repo does not
declare or manage that workspace. See [Cross-module GUI
dependencies](../docs/mf-standards/building-modules.md#cross-module-gui-dependencies) for what
this package's `peerDependencies` shape (below) must satisfy to be workspace-consumable, and
[Building Applications](../docs/mf-standards/building-applications.md#first-time-setup) for the
mechanism itself. `@moduleforge/core-gui` is declared as an **optional** peer dependency here
(`peerDependencies: "*"` + `peerDependenciesMeta.optional`) — under a workspace, bun resolves it
from the workspace member regardless of the declared range or optionality, so this shape does
not need to change to work under a workspace.

## Fresh checkout note — this package's own standalone dev/preview

Running this package's own Ladle stories (`make preview` / `bun run dev`) standalone, outside
any app's workspace, is a separate concern from how an app consumes it above: the stories import
`@moduleforge/core-gui` directly (`ToastProvider`), so `core-gui` must be resolvable even though
the `peerDependencies` declaration is optional. There is no `make link-tags` target — this repo
has no Makefile automation for it — and, as of this writing, no committed lockfile or documented
convention establishes how a module resolves another module's GUI package for its own standalone
preview outside an app workspace; this is tracked as a follow-on decision, not yet standardized
across module repos (see [Building Modules: Cross-module GUI
dependencies](../docs/mf-standards/building-modules.md#cross-module-gui-dependencies), "The
standalone case"). Until that lands, publish-and-link with `yalc` from a sibling `mod-core`
checkout is the working way to make `core-gui` resolvable for a standalone preview session:

```sh
cd ../mod-core/gui && yalc publish   # from a sibling checkout of mod-core
cd -                                  # back to this package's gui/ directory
yalc add @moduleforge/core-gui
```

The resulting `.yalc/` directory is gitignored and must be repopulated after any fresh checkout,
fresh task worktree, or `git clean`.

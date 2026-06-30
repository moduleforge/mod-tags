# mod-tags/gui — deferred component workbench follow-ups

The initial Ladle setup (`gui/.ladle/`, `make preview`) covers the minimum: every exported component has at least one story, Tailwind renders correctly, and HMR works. Items below were intentionally left out of the first pass.

## Story coverage

- Any component added to `gui/src/` after the initial scaffold needs a matching `*.stories.tsx` next to it. Keep stories co-located with their component.
- Variants we punted on:
  - `TagChip` — a long-value overflow case, to verify truncation behavior.
  - `TagList` / `TagEditor` — an explicit error-state snapshot driven by a mock client that rejects `listBySubject`.
  - `TagEditor` — multi-purpose `select` variant with 3+ purposes, to verify the `<select>` branch.

## Stub client vs real API

Stories currently use an in-memory mock `createTagsClient` so they run with zero backend. Once the tags API ships behind a predictable dev endpoint, we can optionally add a second "live" story that points at `http://localhost:<port>` via a Ladle decorator; the mock stories should stay as the default so stories stand alone.

## Storybook migration path

Stories are CSF-compatible, so the swap is config-only:
1. `npm remove @ladle/react` and `npm add -D @storybook/react-vite @storybook/addon-essentials`.
2. Rename `.ladle/` → `.storybook/` and swap `config.mjs` for Storybook's `main.ts` + `preview.ts` (import the same `styles.css`).
3. Keep `vite.config.ts`.
4. Swap the `dev` script: `ladle serve --port 61001` → `storybook dev -p 61001`.

Consider only when tags-gui gains an external audience or a documented design system.

## Addons to turn on when useful

- **a11y**: Ladle 5 ships axe-core integration — flip `addons.a11y.enabled = true` in `.ladle/config.mjs`.
- **MSW decorator**: when real API shapes matter, replace the in-memory mock client with `msw`-intercepted fetch and register an MSW Ladle decorator.

## Visual regression

Same options as mod-core. Playwright over `ladle build` output is the cheapest path.

## Color-editor coverage

`TagChip`'s inline color popover is rendered imperatively (requires user click). Consider a "color-editor-open" story variant that opens the popover on mount via a small wrapper, so the popover UI can be reviewed without interaction. Not a blocker for manual testing.

## Composite stories with core-gui

Tag components currently don't import anything from `@moduleforge/core-gui`, so stories can stand alone. If a future tag component composes core-gui primitives, yalc-link core-gui into `mod-tags/gui` (add a module-level `preview-link` target mirroring the root `link-core` pattern).

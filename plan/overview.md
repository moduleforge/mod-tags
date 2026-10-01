# Migrate mod-tags to a single .flow/ ignore entry

## Purpose and scope

This is wave 2 (wave-2-migration) of the wave plan `flow-ignore-canonicalization`, plan-group `migrate-moduleforge`. Flow now treats `.flow/` as fully git-ignored; this plan migrates the single project `mod-tags` accordingly. One project, one task.

## Overview

Replace every flow-related ignore line in `mod-tags`'s `.gitignore` (inventory: 23:# flow stuff; 26:.flow/*; 27:!.flow/plans; 28:!.flow/project-analysis.json; 29:!.flow/what-next-cache.json) with one `.flow/` entry, and untrack any tracked `.flow` content with `git rm -r --cached .flow` (inventory: 3: .flow/binding.md .flow/project-analysis.json .flow/what-next-cache.json). Files stay on disk. One commit touches only `.gitignore` and the `.flow` untracking.

## Phases

| Phase | Slug | Task | Tier |
|---|---|---|---|
| 15 | flow-ignore-migrate | migrate-gitignore | sonnet-low |

## Risks

The main checkout is clean per the inventory, so close-out merge is not expected to be blocked by uncommitted tracked changes (re-check at close-out).

# Human-friendly page titles

Status: implemented · 2026-09-20

The three routes (`/`, `/applications/`, `/diffs/`) already use plain, sentence-case nav
labels ("Home", "Applications", "Diffs") and lowercase-hyphenated URLs that match those
labels, so no change is needed there. The one gap is the `<title>` tag: all three pages
currently use `Tangle - <Page>` (site name first), which puts the least useful part of
the title where a truncated browser tab shows it. Switching to `<Page> | Tangle`
(specific part first) makes tabs distinguishable when several are open side by side.

## 1. Update the three `<title>` tags

In each `<svelte:head>`, swap `Tangle - <Page>` for `<Page> | Tangle`:

- `web/src/routes/+page.svelte:19` — `Tangle - Home` → `Home | Tangle`
- `web/src/routes/applications/+page.svelte:79` — `Tangle - Applications` →
  `Applications | Tangle`
- `web/src/routes/diffs/+page.svelte:139` — `Tangle - Diffs` → `Diffs | Tangle`

No other file references these exact title strings (`grep -rn "Tangle - "
web/src --include="*.test.ts"` is empty), so this is a self-contained, low-risk edit.

## 2. Verify

- `task ts:lint` (prettier + eslint) and `task ts:test` (existing unit tests) should
  pass unchanged, since no test currently asserts on `<title>` content.
- Manually check each route in the dev server and confirm the browser tab shows the new
  ordering.

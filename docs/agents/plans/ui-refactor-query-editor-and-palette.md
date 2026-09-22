# UI refactor: one query editor, indigo palette, rebuilt Applications and Diffs

Status: in progress · 2026-09-22 — stages 1 and 2 shipped, 3-6 outstanding

Implements the design canvas "Tangle UI — Final design"
(artifact `1f510fad-3738-4e6b-9f5a-cc8f6789bd15`, page *Final design*). The canvas's *Exploration* page holds the
A/B alternatives it was chosen from; only the five `F-*` artboards plus `Palette` are being built.

The canvas is the source of truth for layout and copy. This plan records what it means in this codebase, the
places where it disagrees with what's already here, and the order to build it in.

## What the design asks for

| Artboard | Becomes |
| --- | --- |
| `Palette` | `app.css` primary ramp coral → indigo; `status.ts` five buckets with Badge colours; coral survives as the wordmark only |
| `F-Home` | One `QueryEditor` in a Card with two submits — *See applications* and *See diffs* (disabled until a target ref is typed) |
| `F-Applications` | Page header (H1 + summary + query chips + Edit + Copy link + *Diff these applications*), facet toolbar, sortable scrolling table with a sticky header |
| `F-Diffs` | Same page header, split pane, grouped sidebar with result badges, breadcrumb, tabs (Diff / Target manifests / Live manifests) |
| `F-QueryDrawer` | The same `QueryEditor` in a Drawer, opened by *Edit query* on both pages |
| `F-States` | Skeleton rows, a no-results panel that restates the query, a per-instance failure Alert |

Everything on the boards maps to a stock Flowbite Svelte component, per the canvas's own note — no new UI
primitives, and `flowbite-svelte@1.33.1` is already a dependency.

## Review — where the design meets the code

Five things need deciding or fixing before the boards can be built as drawn. The first two are the ones that
change what gets built.

### 1. The design has no "no query yet" state for Applications, and ADR 0008 requires one

[ADR 0008](../adrs/0008-gate-searches-behind-explicit-user-action.md) gates both pages behind an explicit
search, because a bare nav click to `/diffs` used to fan out one diff-generation POST per application to every
Argo CD. `routes/applications/+page.svelte` implements that gate today with `hasSearched` and the `searched=true`
parameter.

`F-Applications` and `F-Diffs` are both drawn in the *has results* state, and the canvas never shows what
either page looks like when someone clicks the nav link with no query. Taken literally, the new header — which
renders query chips and an *Edit query* button — has nothing to render.

Resolution: **keep the gate, and reuse the drawer for it.** With no query in the URL, the page renders its
header with the H1, an empty-query summary line ("No query yet — pick applications by label") and the *Edit
query* button, and opens the Drawer immediately. That is the same component in the same place, so it costs no
extra design, it honours ADR 0008 (nothing is fetched until the drawer is applied), and it removes the current
oddity where Applications shows a completely different full-page form. ADR 0008 stays accepted; this plan
records the mechanism change, and the `searched=true` parameter is retired since "no labels at all" is now a
legitimate applied query the drawer can submit.

### 2. The partial-failure state cannot be built yet

`F-States`' third panel is drawn with a caveat in the artboard itself, and it is correct:
`applicationsHandler` returns 500 for the whole request as soon as any one instance fails
(`internal/tangle/handlers.go`), so there is no shape in `ApplicationsResponse` that can express "eu-prod
failed, here are the other two".

Resolution: **build the other two states now, leave this one out, and file the API change.** It needs
`ArgoCDApplicationResults` to carry an optional per-instance `error`, the handler to collect failures instead
of returning early, and a decision about what status code a partial success returns. That is backend work with
its own contract question, not part of a UI refactor. The UI keeps today's behaviour (a page-level
`ErrorAlert`) until it lands.

### 3. "Sorted by health, worst first" doesn't work with the current comparator

`sort.ts`'s `sortApplications` does `a[state.key].localeCompare(b[state.key])` on the raw status string, so
sorting by health gives Degraded, Healthy, Missing, Progressing — alphabetical, not severity. The design's
footer says "sorted by health, worst first" and the *Needs attention* facet depends on the same notion.

Resolution: rank statuses by severity in `status.ts` (Degraded/Missing worst, then Progressing, then
OutOfSync, then Unknown/Suspended, then Healthy/Synced) and have `sortApplications` use the rank for the
`health` and `syncStatus` keys, keeping `localeCompare` for `name`.

### 4. Per-key uniqueness is drawn but not enforced

`F-Home` says "Each key once", and the drawer repeats it. `LabelsInput` currently appends any well-formed pair,
duplicates included, and `validation.ts`'s `LABEL_FORMAT` regex only checks shape. The server now rejects a
duplicate key with a 400 ([ADR 0026](../adrs/0026-reject-malformed-label-query-parameters.md)), so today the
UI can build a query the API refuses.

Resolution: enforce it in `LabelsInput` — a key already in the list disables *Add* and explains why — and add
a `duplicateLabelKeys` check to `validation.ts` so a hand-edited URL is caught on load too. The rules mirror
the server's exactly; the server stays the authority.

### 5. `labels.ts` drops malformed pairs silently, like the API used to

`parseLabels` filters out any segment that doesn't split into exactly two parts. A URL like
`?labels=env,team:platform` therefore renders one chip and the user never learns the other was discarded —
the same silent-drop that [#241](https://github.com/ivanklee86/tangle/issues/241) just fixed server-side.

Resolution: have `parseLabels` report what it dropped so the drawer can show it, rather than changing its
signature everywhere at once — a `parseLabelsStrict` returning `{pairs, invalid}` alongside the existing
function, with the existing one kept for callers that genuinely want best-effort.

## Build order

Each stage builds and tests on its own.

### Stage 1 — palette and status vocabulary

- `app.css`: the ten `--color-primary-*` values from coral to Tailwind's indigo ramp.
- `status.ts`: five buckets instead of three, each returning a Flowbite `Badge` colour, an icon and a severity
  rank. Yellow for OutOfSync (drift), red reserved for Degraded/Missing, blue for Progressing, gray for
  Unknown/Suspended, green for Healthy/Synced.
- `Header.svelte`: wordmark keeps coral explicitly, the one place it survives.
- Delete the gradient button classes on `ApplicationsForm`/`DiffsForm` — ADR 0007 already wanted that, and the
  design has no gradients.
- `ArgoCDHealthStatus`/`ArgoCDSyncStatus` render Badges rather than bare coloured icons.

Contrast is the reason for the change, so the tests assert the mapping, not the hex: each status resolves to
the documented colour and rank, and the two status components render the badge text alongside the icon.

### Stage 2 — `QueryEditor`, and Home rebuilt around it

One component, three placements. Props are the query (`labels`, `excludeLabels`, `targetRef`), a `variant`
deciding whether the target ref is required, and a submit callback; the Card/Drawer chrome stays outside it so
the same component serves all three.

- `LabelsInput` gains chips with per-key uniqueness, the `key : value` row with Enter-to-add, and the count
  shown beside the section heading.
- `QueryEditor` adds the target-ref field with its explanatory helper, the link preview with Copy, and the
  `tangle-cli generate-manifests --label env=test --target-ref release-25` preview (flag names verified against
  `cmd/tangle-cli/main.go`).
- `routes/+page.svelte` becomes one Card with two submit buttons, replacing the two duplicated forms.
  `ApplicationsForm` and `DiffsForm` are deleted once nothing imports them.

### Stage 3 — the shared page header, and the drawer

- `QueryBar.svelte`: H1, summary line, query chips, *Edit query*, *Copy link*, and a slot for the page's own
  primary action (*Diff these applications* on Applications, *Reload all* on Diffs).
- `QueryDrawer.svelte`: Flowbite `Drawer` wrapping `QueryEditor`, with the Discard/Apply footer and the change
  count.
- Both pages open the drawer automatically when the URL carries no query (decision 1).

### Stage 4 — Applications

Facet toolbar (Needs attention / All, health and sync `ButtonGroup`s with counts, name filter, auto-refresh
toggle and period), then the table: sticky header, sortable Application/Argo CD/Health/Sync columns, Live ref,
a per-row *Open in Argo CD* link, and the footer counts. Pagination, group-by and the per-row Diff button are
all removed per the design. Row order stays stable across auto-refresh.

### Stage 5 — Diffs

Split pane, sidebar grouped by instance with per-application result badges and a Changed/Errors/All filter,
breadcrumb, the three tabs, and the footer progress bar. `svhighlight` has no split view, so the unified diff
stays — the canvas already reflects that.

### Stage 6 — states and tests

Skeletons, the no-results panel that restates the query, and a pass over the Playwright suites: `web/e2e/mocked`
asserts the new structure against fixtures, `web/e2e/live` stays structural.

## Not in scope

- The partial-failure state and the `/api/applications` change it needs (decision 2) — to be filed separately.
- `j` / `k` / `/` keyboard shortcuts, the one non-Flowbite interaction the canvas notes; they belong after the
  pages exist.
- The four backend follow-ups left open by the label-query work
  ([that plan](label-query-validation-and-exclude-only-selector.md)), one of which — `baseLink` ignoring
  `excludeLabels` — is visible in this design as the per-instance Argo CD deep link.

## Progress

**Stage 1 — done.** `app.css` carries the indigo ramp with the contrast reasoning in a comment; `status.ts` is
five buckets returning a Flowbite `Badge` colour, an icon, a severity rank and a `needsAttention` flag;
`ArgoCDHealthStatus`/`ArgoCDSyncStatus` render Badges; `sort.ts` orders status columns by severity with a name
tiebreak; the wordmark keeps coral; the gradient button classes are gone.

The severity change corrected a test that encoded the old behaviour: `sort.spec.ts` asserted
`['Missing', 'Healthy', 'Degraded']` for a descending health sort — alphabetical, and the opposite of what the
column claims to do. It now asserts worst-first, plus a tiebreak case, because without one the order inside a
severity bucket is whatever order the instances answered in, which changes under auto-refresh.

**Stage 2 — done.** `labels.ts` gained `parseLabelsStrict`, reporting malformed segments and repeated keys
instead of dropping them silently; `validation.ts` gained `isValidLabelQuery` and `duplicateLabelKeys`;
`LabelsInput` enforces per-key uniqueness and reports what a link arrived with that it couldn't use; new
`QueryEditor`, `CopyableText` and `query.ts` (hrefs plus the `tangle-cli` translation, flag names checked
against `cmd/tangle-cli/main.go`); Home is one Card with two actions.

Two corrections made while building:

- **The first `LabelsInput` rewrite lost the failed-add feedback.** Collapsing the two per-field `Helper`s into
  the design's single instruction line meant pressing Enter with an empty value said nothing at all. The helper
  now shows the specific problem when there is one and the instruction only at rest, so the single line in the
  design doesn't cost the feedback the old component had.
- **`ApplicationsForm` had two identically-named Add buttons.** Both its `LabelsInput`s used the default noun,
  so both buttons were "Add label" to a screen reader. Fixed by giving the exclusion input `noun="exclusion"`;
  the component is deleted in Stage 3 regardless, but the same collision would have shipped in `QueryEditor`.

`ApplicationsForm` and `DiffsForm` still exist, because `/applications` and `/diffs` still use them. They go in
Stage 3, along with their tests.

**Stages 3-6 — outstanding.** `QueryBar`, `QueryDrawer`, and the rebuilt Applications and Diffs pages are the
bulk of the visible change and none of it has started. The pages currently render as they did before, on the
new palette and status badges.

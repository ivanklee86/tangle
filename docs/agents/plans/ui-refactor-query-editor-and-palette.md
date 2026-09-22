# UI refactor: one query editor, indigo palette, rebuilt Applications and Diffs

Status: implemented · 2026-09-22

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

Resolution: **keep the gate, and reuse the drawer for it.** With nothing submitted, the page renders its
header with the H1, an empty-query summary line and the *Edit query* button, and opens the Drawer immediately.
That is the same component in the same place, so it costs no extra design, it honours ADR 0008 (nothing is
fetched until the drawer is applied), and it removes the oddity where Applications showed a completely
different full-page form. ADR 0008 stays accepted; this plan records the mechanism change.

**The `searched=true` parameter stays.** An earlier draft of this plan retired it, on the reasoning that "no
labels at all" is a legitimate applied query the drawer can submit — which is true, and is exactly why the
parameter is needed. An empty label filter drops out of the query string, so a submitted empty query and a bare
nav click produce the same URL; without a marker the gate reads both as "nothing asked for" and an empty submit
bounces straight back into the editor, with no way ever to list the whole fleet. `applicationsHref` therefore
emits `searched=true` for an empty query and nothing extra otherwise, and `hasSubmittedQuery` is what the gate
reads. Diffs needs no equivalent: its target ref can only reach the URL by being submitted, so a ref with no
labels is already distinguishable — and it runs, meaning "diff everything against this ref".

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

**Stage 3 — done.** `QueryBar` (H1, summary, query chips, Edit query, Copy link, and a snippet for the page's
own action) and `QueryDrawer` (the `QueryEditor` in a Flowbite `Drawer`, with a working copy of the query, a
change count, Discard/Apply, and the `tangle-cli` preview). The drawer edits a copy and re-seeds on open, so
discarding leaves the page's query alone and a discarded edit doesn't reappear next time.

**Stage 4 — done.** Applications is one table across every instance with an Argo CD column, replacing the
tab-per-instance layout. Facet toolbar (Needs attention / All, health and sync groups with counts, name filter,
auto-refresh toggle and period), sortable columns, sticky header, per-row *Open in Argo CD*, footer counts, and
both empty states — query matched nothing, and filters hid everything. Pagination, group-by and the per-row Diff
button are gone; *Diff these applications* carries the whole query to `/diffs`. `ApplicationsForm` and
`ApplicationsGrid` are deleted.

Decision 1 shipped as planned: `+page.ts` returns `applications: undefined` for an empty query and fetches
nothing, and the page opens the drawer. `load.spec.ts` had three tests asserting that `load` always fetched —
rewritten to assert the opposite, since the whole point of ADR 0008 is that it must not.

Four corrections made while building:

- **`sort.ts` was typed to `ApplicationLinks`,** which has no instance, so the Argo CD column couldn't be
  sorted. It is now generic over a narrow `SortableApplication`, with `instance` added to `SortKey`.
- **Two "Clear filters" buttons.** With every row filtered out, the empty state and the table footer both
  offered one — an ambiguous accessible name and a worse answer than one button. The footer's only renders
  while something is still on screen.
- **The loading region had an `aria-label` and no text.** A live region announces its *contents*, so a label
  alone leaves a screen reader with nothing to read; it now carries `sr-only` text.
- **Two e2e races and a hidden checkbox.** `allTextContents()` doesn't auto-wait, so the row-order assertion
  read an empty table; and Flowbite's `Toggle` hides the real checkbox behind a styled span, so Playwright's
  actionability check never passes without `force`.

**Stage 5 — done.** Diffs is a `SplitPane`: a sidebar listing every application grouped by instance with an
outcome dot, `+n −n` counts and a Changed/Errors/All filter, and a detail pane with the breadcrumb, status
badges, `liveRef → targetRef`, Reload diff, Open in Argo CD, and the three tabs. `$lib/ui/diffs.ts` holds the
model — outcome classification, line counts, filtering, grouping and selection — so the page stays layout.
Rows render as soon as the applications call returns and show `pending` until their own diff arrives, which is
what makes the sidebar useful during a long fan-out. `DiffsForm` and `AppManifests` are deleted.

**Stage 6 — done.** Both Playwright suites rewritten for the new pages: the mocked suite asserts structure
against fixtures, the live suite stays structural per [ADR 0003](../adrs/0003-svelte-e2e-testing-strategy.md).
Skeleton loading and both empty states ship on each page. The per-instance failure state is still out (decision
2) and still needs the API change.

### What running it against the live stack found

Two things no test would have caught, because both were invisible until a page displayed the value.

**`ApplicationLinks.LiveRef` was tagged `json:"LiveRef"`** while every sibling field — and the web UI's own
`ApplicationLinks` interface, and the Playwright fixtures — used lowerCamelCase. The frontend therefore read
`undefined` for it, always. Nothing noticed because nothing rendered it until this design added a Live ref
column; `buildDiffRequests` had been sending `liveRef: undefined` in every diff POST. Fixed to `json:"liveRef"`,
with `TestApplicationsResponseJSONShape` pinning the whole wire shape so it can't drift again.

Worth noting *how* it hid: `e2e/fixtures/applications.json` spelled the field `liveRef`, the way the UI wanted
rather than the way the server sent it. The mocked suite was green against a response shape the server never
produced — exactly the fixture drift ADR 0003 predicted, caught exactly the way that ADR intended.

**The unsortable Live ref header rendered uppercase while the sortable ones didn't,** because Flowbite's
`TableHeadCell` uppercases its own content but the `<button>` inside the sortable cells reset it.

### Not built as drawn

- **No per-resource breakdown in the Diff tab.** The design shows "Diff 2 resources" and an accordion per
  Kubernetes resource. `diffManifests` (`internal/tangle/manifests.go`) shells out to `diff -uNar` over two
  temp files holding the whole manifest set, so the response is one unified diff with no resource boundaries
  to recover. The tab shows `+n −n` instead, which is derivable and honest.
- **The diff's file headers are temp paths** (`--- /tmp/tangle…/live_<uuid>.yaml`) rather than the design's
  `--- live` / `+++ target`, for the same reason: they come from the server's `diff` invocation. A `--label`
  flag pair there would fix it. Backend, and not part of this refactor.
- **No `j`/`k`/`/` shortcuts**, as planned.

## Follow-up: the empty-query gate was broken on first delivery

Reported after stages 5-6 landed: submitting the Applications editor with nothing filled in should list every
application, and didn't.

The cause was decision 1 as originally written. Retiring `searched=true` removed the only thing distinguishing
a submitted empty query from a bare nav click — both are `/applications` — so applying the drawer with no
labels navigated to a URL the gate read as "nothing asked for", which reopened the drawer. There was no route
to an unfiltered listing at all.

Fixed by restoring the marker in `query.ts`: `applicationsHref` emits `searched=true` only when the query is
otherwise empty, and `hasSubmittedQuery` is what both `+page.ts` files consult instead of `isEmptyQuery`.
Diffs had the same trap for a ref with no labels; its gate now reads the target ref alone, since a ref can only
get into the URL by being submitted. `QueryBar` distinguishes the two empty states in its own copy — "none yet
— pick applications by label" versus "no filters — showing everything" — because they no longer mean the same
thing.

Covered at every layer, since the bug lived in the seam between them: `query.spec.ts` on the marker and the
round trip through `hasSubmittedQuery`, both `load.spec.ts` files on what does and doesn't fetch, both page
tests on applying an empty query and on rendering one, and a mocked e2e that submits the empty form on Home and
checks the fleet is listed. Verified against the live stack too.

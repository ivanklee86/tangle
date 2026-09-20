---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Require explicit user action before Applications/Diffs run a search or diff fan-out

## Context and Problem Statement

`routes/diffs/+page.svelte` fetched applications on `load()` and then, in a client-side `$effect`, always
called `fetchDiffs()` — which fans out one diff-generation POST per application across every ArgoCD instance
(`lib/backend/diffs.ts`'s `buildDiffRequests`, fully parallel via `Promise.all`, no concurrency cap). If the
page loaded without a `targetRef` query parameter — which happens on a bare visit to `/diffs`, e.g. via the nav
bar link — `buildDiffRequests` silently fell back to comparing each application's own live ref against itself.
That is a request per application, to every ArgoCD instance, to produce a diff that is always empty by
construction: pure waste, and at scale it means clicking the wrong nav link "hammers" every connected ArgoCD
for nothing.

`routes/applications/+page.svelte` had a milder version of the same shape: `load()` always fetched the
applications list immediately, regardless of whether the user had specified any label filter, so landing on
the tab ran an unfiltered "show everything" search by default rather than waiting for intent.

## Decision Drivers

- The diffs fan-out is the expensive case (N requests, each triggering real manifest generation against
  ArgoCD/git) and was firing on a code path — a bare nav-link click — that has no `targetRef` to give it, by
  construction. This isn't a rare misuse; it's the default outcome of the most obvious way to reach the page.
- The home page (`routes/+page.svelte`) already had a `labels`/`excludeLabels`(/`targetRef`) form pattern for
  both actions — reusing it instead of inventing a second UI idiom for the same idea kept the fix consistent
  rather than one-off.
- `+page.ts`'s `load()` for both routes only ever reads `labels`/`excludeLabels` (diffs also reads nothing for
  the fan-out itself — `targetRef` is read client-side); the actual expensive work is entirely client-side and
  triggerable independently of `load()`, which is what makes gating it there — without touching `load()` — the
  narrowest fix available.

## Considered Options

- Leave both routes as-is and rely on users not bookmarking or nav-clicking into a bare `/diffs`.
- Gate only the diffs page (the objectively expensive N-way fan-out) and leave `/applications`'s single,
  comparatively cheap GET running immediately on load.
- Gate both routes behind the same explicit-submission pattern: extract the home page's two forms into shared
  `ApplicationsForm`/`DiffsForm` components, and show the relevant one on each route until the user submits it
  (or arrives with the relevant filter already in the URL, e.g. from the home page).

## Decision Outcome

Chosen option: gate both routes behind shared, reused form components. `lib/ui/components/ApplicationsForm.svelte`
and `DiffsForm.svelte` hold their own submit-button-disabled-until-valid state (the target ref requirement on
Diffs, previously enforced with a `noRefSpecified` toast, is now enforced by the submit button simply staying
disabled — "design, not text") and call an `onSubmit` callback the parent route wires to its own navigation:

- `routes/diffs/+page.svelte`: shows `DiffsForm` whenever `targetRef` is absent from the URL; the `$effect`
  bails out before calling `fetchDiffs` in that case, so zero diff requests fire. Submitting calls `goto()`
  with the new `targetRef`/labels in the query string.
- `routes/applications/+page.svelte`: shows `ApplicationsForm` unless the URL already indicates an explicit
  search — either a real `labels`/`excludeLabels` value, or a `searched=true` marker. That marker exists
  specifically because `labels`/`excludeLabels` are legitimately optional: `buildQuery` (`lib/backend/url.ts`)
  drops empty values, so a search submitted with both fields blank produces a URL indistinguishable from a bare
  nav-link click unless something else marks it as submitted. This follows the same precedent as the page's
  existing `refresh=true` query parameter.
- The home page's own two forms became instances of these same components, so all three routes share one
  implementation of "ask before you search."

### Consequences

- Good, because the diffs page can no longer fan out N requests to every ArgoCD instance as a side effect of
  navigation — it always requires an explicit, non-empty target ref first.
- Good, because all three routes (`/`, `/applications`, `/diffs`) now present the same form for the same
  action, instead of the home page's version being the only "real" one.
- Good, because `+page.ts`'s `load()` functions and their tests (`load.spec.ts`) needed no changes — the fix is
  entirely about when the client-side effect/render decides to act, not what the loaders fetch.
- Neutral, because `load()` still fetches the applications list unconditionally on `/applications` even while
  the form is showing (it's a single cheap GET, and having it ready removes a second load-then-fetch round trip
  once the user does submit) — the gate is on rendering/acting, not on that particular request.
- Bad, because the `searched=true` marker is a small URL wart specific to the applications route's optional
  filters; it has no equivalent need on the diffs route, where `targetRef` itself is a sufficient, always
  non-empty signal once submitted.

## Pros and Cons of the Options

### Leave as-is

- Good, because zero effort.
- Bad, because it leaves the actual reported problem (bare nav click into `/diffs` fans out N requests to every
  ArgoCD instance) in place.

### Gate only Diffs

- Good, because it fixes the objectively expensive case with a smaller diff.
- Bad, because it leaves `/applications` behaviorally inconsistent with `/diffs` for no reason other than one
  being cheaper — the request was explicitly for both to behave "logically the same."

### Gate both via shared form components (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [Gate Applications/Diffs behind explicit search](../agents/plans/gate-applications-and-diffs-searches.md)
- Related: [ADR 0007](0007-flowbite-design-system-consistency.md) (the visual-consistency pass from the same
  review; this ADR covers a distinct, functional decision about request timing rather than appearance)

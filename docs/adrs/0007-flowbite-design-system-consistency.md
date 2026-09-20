---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Treat Flowbite Svelte and its theme tokens as the frontend's single source of visual truth

## Context and Problem Statement

[ADR 0002](0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md) upgraded `web/` to Tailwind
CSS v4 and `flowbite-svelte` 1.x as a pure version bump, carrying the existing markup across mechanically with
no visual changes intended. Reviewing that markup afterward surfaced a set of small inconsistencies that had
accumulated across the app: `<br />` used as a layout primitive instead of Tailwind spacing utilities;
Flowbite's semantic `Alert`/`Card` color props overridden with raw Tailwind classes in a way that actually
broke dark-mode styling (`tailwind-merge` replaces the light-mode classes but leaves the theme's `dark:`
classes in place, producing a saturated box in light mode and a washed-out one in dark mode); the same
"system error" alert markup duplicated across three files; per-status icon/color mapping duplicated and
disagreeing between the two ArgoCD status components; page headings sometimes using Flowbite's `Heading`
component and sometimes a hand-styled `<h5>`; a `Navbar color="form"` prop that does nothing (`Navbar` has no
`color` prop in 1.x — it silently falls through to a DOM attribute); and every prominent call-to-action using a
stock `GradientButton color="pinkToOrange"` gradient with no relationship to the custom coral
`--color-primary-*` ramp defined in `app.css`. None of this was a functional bug, but left unaddressed each of
these was a template for the next component to copy.

## Decision Drivers

- AGENTS.md's testing philosophy of verifying observable behavior implies a parallel expectation for the
  visual layer: a consistent design language should be enforced by convention, not left to accumulate drift
  component by component.
- `app.css` already defines a real brand palette (`--color-primary-*`, coral `#fe795d` at 500) that most of the
  UI wasn't using — the highest-leverage fix available was making the app actually use the token investment
  already made, rather than inventing a new one.
- `tailwind-merge` (which Flowbite Svelte's `tv()`-based theming uses internally) makes a raw-Tailwind override
  of a semantic color prop something worse than redundant — as observed with the red alert above, it can
  silently break the light/dark pairing a component's theme was designed to keep together.
- The project has no visual regression suite (noted as a known gap in ADR 0002), so preventing drift up front
  is cheaper than catching it later.

## Considered Options

- Leave the inconsistencies as found and address them opportunistically as each is next touched.
- Migrate off Flowbite Svelte to a different component library as part of fixing this.
- Formalize Flowbite Svelte's components and `app.css`'s theme tokens as the single source of visual truth,
  and fix the concrete inconsistencies found under that rule.

## Decision Outcome

Chosen option: formalize Flowbite Svelte + `app.css` theme tokens as the source of truth, with a concrete rule
against overriding a component's semantic color prop with raw Tailwind color classes, and fix the
inconsistencies found. Specifically:

- Every layout gap uses a Tailwind spacing utility (`space-y-*`, `mt-*`/`mb-*`) on a container or element —
  `<br />` is reserved for its actual purpose (a line break inside a run of text), not page layout.
- The duplicated "System error!" alert became one component, `lib/ui/components/ErrorAlert.svelte`
  (`<Alert color="red" border>`, no class override), used by `ApplicationsGrid.svelte`, `AppManifests.svelte`,
  and `routes/diffs/+page.svelte`.
- The duplicated, disagreeing status→icon/color mapping in `ArgoCDHealthStatus.svelte`/`ArgoCDSyncStatus.svelte`
  became one helper, `lib/ui/status.ts`'s `statusAppearance()`, with both components reduced to thin wrappers
  and a single, deliberate fallback (amber "unknown") instead of two different accidental ones.
- All page/card headings use Flowbite's `Heading` component; hand-rolled heading markup was removed.
- The dead `Navbar color="form"` prop was removed; primary call-to-action buttons switched from
  `GradientButton color="pinkToOrange"` to a `Button color="primary"` with a `bg-gradient-to-br
  from-primary-400 to-primary-700` overlay — colorful, but built from the theme's own ramp instead of an
  unrelated stock preset.
- Nav-link active-state highlighting (`NavUl`/`NavLi` in `Header.svelte`) was fixed to actually work — the
  `href`s needed a trailing slash to match `page.url.pathname` under this app's `trailingSlash = 'always'`
  config, which had silently made two of the three nav links permanently non-highlightable — and restyled to
  `font-bold text-black dark:text-white` (a legibility choice made after visual review, not the initial
  primary-colored-background attempt).

### Consequences

- Good, because the app now actually uses the brand palette it already pays the token-maintenance cost for,
  and retheming (changing `--color-primary-*`) now visibly affects the whole app, not just a few buttons.
- Good, because the shared `ErrorAlert` and `statusAppearance()` mean the next error surface or status value
  gets consistent treatment for free instead of by copy-paste.
- Neutral, because this is a convention, not something enforced by tooling — a future PR can still add a raw
  Tailwind color override; there is no lint rule for it (see More Information for a possible follow-on).
- Bad, because Flowbite's `secondary` color (on `Button`/`Badge`/`Alert`) has no backing `--color-secondary-*`
  tokens in `app.css` or `flowbite/plugin`, so it renders unstyled if ever reached for — a trap for a future
  contributor that this decision does not close, only documents.

## Pros and Cons of the Options

### Leave as-is, fix opportunistically

- Good, because it requires no dedicated effort right now.
- Bad, because "opportunistic" is exactly how the inconsistencies accumulated in the first place — nothing
  about touching one file next incentivizes fixing an unrelated one.

### Migrate off Flowbite Svelte

- Good, because it would be a chance to pick a smaller-surface library if Flowbite's pace becomes a problem.
- Bad, because it is the same much-larger, unrelated migration ADR 0002 already rejected for this reason, and
  none of the problems found here are caused by Flowbite itself — they're caused by not using it consistently.

### Formalize Flowbite + theme tokens as source of truth (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [Flowbite design consistency pass](../agents/plans/flowbite-design-consistency.md)
- Related: [ADR 0002](0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md) (the upgrade this
  pass follows), [ADR 0008](0008-gate-searches-behind-explicit-user-action.md) (a related but distinct
  decision from the same review, about *when* the UI performs network actions rather than *how* it looks)
- A lint rule or code-review checklist item against raw-Tailwind color-class overrides on Flowbite components
  would make this decision self-enforcing instead of convention-only; not implemented here.

---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Replace the free-text Labels field with a structured key/value input

## Context and Problem Statement

[Issue #137](https://github.com/ivanklee86/tangle/issues/137) reports that the Labels field is a single
free-text box the user has to type a correctly-formatted `key:value,key2:value2` string into, and that this is
confusing. `ApplicationsForm.svelte` and `DiffsForm.svelte` each render this as one `<Input>` bound to a plain
`labels: string` / `excludeLabels: string`, placeholder `"Labels in format 'key:value'"`, validated only by
`isValidLabelFormat()` (`web/src/lib/ui/validation.ts`) — a regex that gates the submit button on the whole
string being well-formed, but gives no feedback about which particular pair is wrong, and never parses the
string into a structure; it stays an opaque string all the way from the input to the URL query string. The
actual parse into a `map[string]string` happens server-side, in `internal/tangle/handlers.go`'s
`applicationsHandler` (`strings.Split(..., ",")` then `strings.Split(pair, ":")`) — so the `key:value,...`
string is a wire contract between the frontend and this handler, not just a UI convenience the frontend is free
to change unilaterally.

The issue's own suggested fix is explicit: "Turn it into two boxes (key, value) with option to add more with a
`[+]` button." Before committing to building that, we checked whether `flowbite-svelte` (this project's
component library, per [ADR 0007](0007-flowbite-design-system-consistency.md)) already ships something that
solves this with less custom code: it has a `Tags` component (`forms/tags/Tags.svelte`) — a bindable
`value: string[]` chip input where typed text becomes a removable chip on Enter. It would let a `key:value`
pair be typed once and then handled as a single removable unit, but it's still one text box the user types
`key:value` syntax into — it doesn't give the two separate boxes the issue asks for.

## Decision Drivers

- The issue's explicit ask is two boxes plus a `[+]` button — a structured key/value entry, not merely a nicer
  single free-text box.
- `handlers.go`'s parser already forbids `:`/`,` inside a key or value; nothing in the issue asks for that to
  change, and loosening it would also touch `pkg/client/client.go`'s `ApplicationsUrlOptions` and any consumer
  of that Go package — a materially bigger change than the UI request in front of us. Scope this decision to
  the frontend only.
- `ApplicationsForm.svelte` and `DiffsForm.svelte` already duplicate the exact same Labels/Exclude Labels
  markup verbatim (the same duplication [ADR 0008](0008-gate-searches-behind-explicit-user-action.md) accepted
  when it extracted these two components in the first place, scoped to the whole form rather than this one
  field) — a new field implementation should be a single shared component both forms use, not two more copies.
- ADR 0007 established flowbite-svelte components and `app.css`'s theme tokens as the frontend's single source
  of visual truth — any new interactive element should be assembled from existing Flowbite primitives
  (`Input`, `Button`, `Badge`), not a one-off hand-rolled widget.

## Considered Options

- Leave the single free-text `<Input>` as-is.
- Adopt flowbite-svelte's `Tags` component as a near-drop-in replacement: one text box, `key:value` typed as
  text, Enter commits it as a removable chip.
- Build a custom two-box (`key` input + `value` input + add button) component, rendering committed pairs as
  removable chips, as a new shared `LabelsInput.svelte`.

## Decision Outcome

Chosen option: a custom two-box-plus-chips component, frontend-only.

New `web/src/lib/ui/components/LabelsInput.svelte` owns its own `Label` heading and renders a `key` `Input`, a
`value` `Input`, and a `PlusOutline` add button, plus one `Badge dismissable` chip per committed pair (reusing
`Badge`'s built-in `close` event rather than a hand-rolled dismiss button). Externally it keeps the exact same
contract the plain `<Input bind:value={labels}>` had — a single bindable `value: string` in the existing
`key:value,key2:value2` wire format — so `ApplicationsForm.svelte`/`DiffsForm.svelte`'s own `labels`/
`excludeLabels` state, `isValidLabelFormat`-based `canSubmit`, `onSubmit` callback signature, `buildQuery`, and
`handlers.go` all stay unchanged; only the field's internal markup and interaction model change. Per-pair
validation before a pair is added reuses `isValidLabelFormat` on the composed `key:value` candidate string,
rather than introducing a second, parallel format check.

### Consequences

- Good, because it matches the issue's literal request — two boxes and a `[+]` button — rather than a
  different UI that happens to also reduce typing errors.
- Good, because a malformed pair is now rejected (add button disabled) at the point of entry, one pair at a
  time, instead of the whole string silently failing a submit-button-disabling regex with no indication of
  which pair is wrong.
- Good, because it's a drop-in replacement at the `bind:value` boundary — zero changes needed to
  `isValidLabelFormat`, `canSubmit`, `onSubmit`, `buildQuery`, or `handlers.go`.
- Good, because extracting one shared `LabelsInput.svelte` (used by both `Labels` and `Exclude Labels` in both
  forms) removes the field-level markup duplication between `ApplicationsForm.svelte` and `DiffsForm.svelte`.
- Neutral, because the `:`/`,` restriction inside a key or value is unchanged from today — it's still a wire-
  format limitation the user can hit, just no longer something they have to get right by hand-typing
  delimiters.
- Bad, because this is more custom code and more component tests to write and maintain than adopting `Tags`
  as-is would have needed — `Tags`' Enter/Backspace chip affordances are effectively hand-rolled again here for
  the two-box case, rather than reused from the library.

## Pros and Cons of the Options

### Leave the free-text `<Input>` as-is

- Good, because it requires no effort.
- Bad, because it leaves the reported confusion in place — the field the issue is specifically about.

### Adopt flowbite-svelte's `Tags` component

- Good, because it's a bindable `string[]` component already in the library, needing very little custom code,
  and gives visible, individually-removable chips for free.
- Bad, because it's still one text box the user types `key:value` syntax into to create each chip — it doesn't
  give the two separate boxes the issue explicitly asks for, and doesn't expose a per-tag validation hook, so
  a malformed pair would need to be caught after the fact (reactively stripped/flagged) rather than rejected at
  entry.

### Custom two-box + chips component (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [Structured key/value Labels input](../agents/plans/structured-key-value-labels-input.md)
- Related: [ADR 0007](0007-flowbite-design-system-consistency.md) (Flowbite as the frontend's source of visual
  truth, which this decision's chip/button choices follow), [ADR 0008](0008-gate-searches-behind-explicit-user-action.md)
  (the shared-form-component precedent this decision extends down to the field level)

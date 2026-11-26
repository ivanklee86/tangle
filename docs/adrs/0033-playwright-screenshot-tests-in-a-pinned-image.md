---
status: "accepted"
date: 2026-11-26
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Compare screenshots with Playwright, in the pinned Playwright image, on a fixture fleet

## Context and Problem Statement

Exploratory testing of the multi-source UI (#292) found visual regressions that passed every unit, component, mocked and live test: a dark-mode row background that didn't exist in the palette, a table overflowing its container, alerts unreadable in dark mode, and a checked checkbox drawn invisible. Those tests assert text and roles, so they can't see colour or layout. Separately, the README's screenshots came from a script that needed the live stack, showed the integration fixtures (`test-1`, `env:test`), and sometimes had to be touched up by hand to look realistic.

How do we catch visual regressions in CI, and keep the documentation screenshots honest, without flaky pixel comparisons or a new service?

## Decision Drivers

- No new hosted service and no new test framework; we already run Playwright.
- A baseline must compare equal on every run, locally and in CI.
- Updating baselines after an intended change, or a dependency update that moves pixels, has to be one command.
- The documentation screenshots should show a believable fleet and regenerate without a backend.

## Considered Options

- Playwright `toHaveScreenshot()` on mocked fixtures, inside the pinned Playwright Docker image (chosen).
- The same, on the host.
- Vitest browser mode `toMatchScreenshot()` on components.
- A hosted service (Argos, Lost Pixel, Chromatic).

## Decision Outcome

Chosen option: **Playwright `toHaveScreenshot()` in `mcr.microsoft.com/playwright:v<pinned version>-noble`, on a fixture fleet**, because it reuses the browser automation we have and only adds a rendering environment.

- `web/e2e/fixtures/fleet.ts` is a realistic fleet with a diff responder that follows ADR 0032's planning rules, so screens behave as against a real server and nothing varies between runs.
- `web/playwright.visual.config.ts` has two projects: `visual` compares screenshots (`web/e2e/visual/`, baselines in `__screenshots__/` next to the spec), and `docs` writes `docs/images/Tangle{Home,Applications,Diffs}.png`.
- `web/scripts/playwright-docker.sh` runs either inside the image whose tag matches the Playwright in `package-lock.json`. It runs as the host user, and keeps the container's `node_modules` and `.svelte-kit` on tmpfs so the host checkout and a running `vite dev` are untouched.
- `task ts:test:visual`, `task ts:test:visual:update` and `task ts:screenshots` wrap it. CI's `visual` job runs it on the amd64 runner only.

### Consequences

- Good, because the regressions above now fail CI with an expected, actual and diff image as an artifact. Reintroducing the dark-mode row background fails exactly the two screenshots containing a moved row.
- Good, because the documentation screenshots are reproducible, realistic and need no backend.
- Bad, because UI changes and Renovate's Flowbite or Tailwind updates will move pixels. Those PRs need `task ts:test:visual:update` and a look at the new baselines.
- Bad, because the screenshot tests need Docker, locally as well as in CI.
- Neutral, because baselines are only valid in the image. A Playwright bump changes the image tag and is likely to need a baseline update in the same PR.

### Confirmation

- Regenerating then comparing passes repeatedly in the image.
- Reintroducing a known regression (`dark:bg-primary-950/40` in the Sources panel) fails the affected screenshots.

## Pros and Cons of the Options

### The same on the host

- Good, because there's no Docker.
- Bad, because fonts and anti-aliasing differ between machines and between CI's amd64 and arm64 runners, so baselines would fail anywhere but where they were made.

### Vitest browser mode `toMatchScreenshot()`

- Good, because component-level shots are small and focused.
- Bad, because several of the regressions were page layout (overflow, wrapping) that a component in isolation doesn't show, and it has the same rendering-environment problem. It's a possible follow-up, not a replacement.

### A hosted service

- Good, because of review dashboards and approval flows.
- Bad, because it's a new account, a dependency and a cost, for a repo whose PRs are reviewed by one or two people.

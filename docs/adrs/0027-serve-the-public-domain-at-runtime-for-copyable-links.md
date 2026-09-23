---
status: "proposed"
date: 2026-09-22
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Serve the public domain at runtime so the web UI can build links worth copying

## Context and Problem Statement

The query editor offers a link for every query, meant to be pasted into a ticket or a chat message. It showed a path — `/applications?labels=env:test` — which nobody else can open. Two inches away in the same page header, *Copy link to this view* copied `window.location.href`, a full URL. The same screen offered two copy buttons producing different kinds of thing.

Making the preview absolute needs a base URL, and the frontend has no obvious source for one. Three things were already in the tree and none of them worked:

- **`TangleConfig.Domain`** is declared (`koanf:"domain"`), and `integration/tangle.yaml` set it to `https://tangle.test.com`. Nothing read it. It was configured, documented nowhere, and dead.
- **`TANGLE_DOMAIN`** already overrode it, because [ADR 0023](0023-resolve-env-var-overrides-against-the-config-schema.md)'s environment plumbing reflects over `TangleConfig` and accepts any tagged key. The environment-variable half of the feature was built; the value had no consumer.
- **`PUBLIC_BASE_URL`** is a SvelteKit build-time variable baked into the static bundle. `web/.env.production` leaves it empty so API calls stay same-origin. One container image serves every deployment, so it can never carry a deployment's own hostname.

`window.location.origin` is available to the browser for free and is, for a copy button, unusually well-suited: it is by construction the address that worked for the person copying. It fails in exactly one situation — somebody reaching Tangle through a `kubectl port-forward` sees `localhost:8081`, and the link they copy is useless to a colleague. That situation is common enough to be the reason this question came up.

## Decision Drivers

- A copied link has to work for the person receiving it, not the person sending it.
- One image serves many deployments, so anything deployment-specific has to arrive at runtime.
- The common case should need no configuration at all.
- Operators should find out about a mistyped value immediately, not from a colleague reporting a dead link.
- The two copy affordances on one screen should produce the same kind of thing.

## Considered Options

- Browser origin only.
- A configured domain only, required.
- Browser origin by default, with a configured domain overriding it — delivered by a runtime `GET /api/config`.
- The same two-tier rule, delivered by templating the value into `index.html` as the Go server serves it.
- The same two-tier rule, delivered by a build-time variable or `<meta>` tag.

## Decision Outcome

Chosen option: **browser origin by default, with a configured `domain` overriding it, delivered by a runtime `GET /api/config`**, because it makes the common case free and correct while giving the port-forward case — the only one the browser cannot reason about — a way out.

Three sub-decisions come with it:

**A configured domain always wins when set.** Not "only when the origin looks local", which is cleverer and less predictable. Controlling the output is the reason to set the value.

**A malformed domain stops the server at startup**, rather than being logged and ignored. It can only be a typo in configuration the operator owns, it is cheap to catch at load, and the failure it would otherwise cause is silent: every copied link points somewhere wrong, which is worse than no link. This is deliberately the opposite of how [#234](https://github.com/ivanklee86/tangle/issues/234) treats unknown `TANGLE_`-prefixed environment variables — those are ignored precisely because they are *not* ours to validate, since Kubernetes injects them into any pod sharing a namespace with a Service named `tangle`. The distinction is ownership of the input, not severity.

**`/api/config` carries only `domain`, but is named for the general case.** A `/api/domain` would have to be replaced the first time the UI needs a version string or instance metadata.

### Consequences

- Good, because an unconfigured Tangle produces correct absolute links with no setup, and a port-forwarded one can be made to produce shareable ones with a single setting.
- Good, because `domain` stops being dead configuration: it is now read, validated, documented in `docs/configuration.md`, and settable as `TANGLE_DOMAIN` with no new plumbing.
- Good, because the frontend has somewhere to put the next runtime-only value.
- Bad, because every page load makes one extra same-origin request, and the root layout awaits it. Streaming it would avoid the wait but render a path that turns into a URL a moment later, which is a worse thing to look at than a few milliseconds of nothing.
- Bad, because a new failure mode exists — a typo'd `domain` now prevents startup. That is the intended trade, and the message names the offending value.
- Neutral, because an older server without `/api/config` degrades silently to the browser origin: a non-OK response, a thrown request and an unexpected body all yield an empty domain.

## Pros and Cons of the Options

### Browser origin only

- Good, because it needs no server work, no configuration, and no extra request.
- Good, because it is right for everyone whose address bar shows the address they would share.
- Bad, because it cannot help the port-forward case at all, and that case is the reason the question was asked.

### A configured domain only, required

- Good, because the output is fully predictable and there is one code path.
- Bad, because it makes a link preview depend on configuration that most deployments would never need, and gets the answer *wrong* — with no fallback — the moment the setting is missing or stale.

### Templating the domain into `index.html` at serve time

- Good, because it avoids the extra request entirely.
- Bad, because `build/index.html` is `adapter-static`'s fallback, served for every route, and rewriting it on the way out couples the Go file server to the frontend's build output. Fragile for what it saves.

### A build-time variable or `<meta>` tag

- Good, because it is the least code.
- Bad, because it does not work. One image is built once and serves every environment; this is the same reason `PUBLIC_BASE_URL` is empty in production.

## More Information

Implemented in `internal/tangle/loader.go` (`normalizeDomain`), `internal/tangle/handlers.go` (`configHandler`), `web/src/lib/backend/config.ts` and `web/src/lib/ui/links.ts`. The plan, including the staging and the `integration/tangle.yaml` hazard it had to clear, is in [docs/agents/plans](../agents/plans/absolute-links-and-configured-domain.md).

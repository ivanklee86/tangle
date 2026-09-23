# Absolute links in the query preview, and a domain the server actually uses

Status: implemented · 2026-09-22

The Link preview in the query editor shows a path (`/applications?labels=env:test`), not a URL. This plan
covers making it absolute, and what "get the hostname" should mean.

## What exists today

**Yes, there is a way — client-side, and it's one line.** `window.location.origin` is the URL the person is
already looking at, which for a "copy this link" affordance is the most correct value available: it is by
construction the address that worked for them.

Three more things are already in place, which changes the shape of the request:

- **`TangleConfig.Domain` already exists** (`koanf:"domain"` in `internal/tangle/config.go`) and
  `integration/tangle.yaml` sets it to `https://tangle.test.com`. **Nothing reads it.** It is declared,
  configured, and dead — the only other `Domain` in the tree is `pkg/client`'s unrelated CLI flag.
- **`TANGLE_DOMAIN` already overrides it.** The env plumbing from
  [ADR 0023](../adrs/0023-resolve-env-var-overrides-against-the-config-schema.md) reflects over
  `TangleConfig`, so any tagged key is already settable from the environment. The env-variable half of the
  request is built; the value simply has no consumer.
- **`PUBLIC_BASE_URL` cannot do this job.** It is a *build-time* variable baked into the static bundle
  (`web/.env.production` leaves it empty so API calls stay same-origin). One image serves every deployment, so
  it can't carry a per-deployment hostname.

**There is also an inconsistency inside the current UI.** `QueryBar`'s *Copy link to this view* copies
`window.location.href` — already absolute. `QueryPreview`'s LINK block shows and copies a path. The same screen
offers two copy buttons that produce different kinds of thing.

## Recommendation: both, in that order

Use `window.location.origin` by default, and let a configured domain override it.

Origin alone fixes the inconsistency today, costs nothing, needs no configuration, and is right whenever the
address someone used is the address they'd share. It is *not* enough in one real case: someone reaching Tangle
on `localhost:8081` through a port-forward copies a link that is useless to a colleague. That case is the
entire argument for server-side configuration, and it is a good one — it just shouldn't be the only path.

## How the browser would learn a configured domain

The frontend is a static bundle served by the Go server, so this has to arrive at runtime.

| Option | Verdict |
| --- | --- |
| `GET /api/config` returning `{"domain": "..."}`, fetched once in the root `+layout.ts` | **Recommended.** Fits the existing API surface and swagger generation, needs no build changes, and gives a place for the next runtime value that comes up. Costs one small request per page load. |
| Template the domain into `index.html` at serve time | Avoids the request, but means the Go server rewrites a file `adapter-static` emitted, and `build/index.html` is a fallback used for every route. Fragile for the saving. |
| A `<meta>` tag written at build time | Same defect as `PUBLIC_BASE_URL` — one image, many deployments. |

## Staging

**Stage 1 — absolute links, no server work.** `QueryPreview` renders `window.location.origin + href`. This is
the whole visible fix for anyone not behind a port-forward, and it makes the two copy buttons agree. Independent
of everything below, and worth shipping on its own.

**Stage 2 — make `domain` real.** Validate it at load (absolute, has a scheme, trailing slash stripped),
document it in `docs/configuration.md` where it is currently absent, and serve it from `/api/config`. Decide
what an invalid value does — see below.

**Stage 3 — prefer the configured value.** The frontend uses the configured domain when the server supplies
one, falling back to `window.location.origin`. One helper, `absoluteUrl(href)`, so no component makes this
decision itself.

## Decisions this needs

- **Does a configured domain always win, or only when the origin looks local?** Recommend always. "Only when
  local" is cleverer and less predictable, and the reason to set the value is to control the output.
- **What does an invalid `domain` do?** Recommend failing at startup: it is a typo in configuration, it is
  cheap to catch, and a silently ignored one produces links that are wrong rather than absent. Contrast with
  [#234](https://github.com/ivanklee86/tangle/issues/234)'s handling of unknown env vars, where ignoring was
  right because the input wasn't ours to validate.
- **Does `/api/config` carry anything else?** Keep it to `domain` now, but name it generically rather than
  `/api/domain`, so version or instance metadata has somewhere to go later.

## Watch out

**`integration/tangle.yaml` already sets `domain: "https://tangle.test.com"`.** Today that is inert. The moment
Stage 2 lands, the integration and live-e2e environment starts producing `https://tangle.test.com/...` links —
a hostname that does not resolve. Either change that fixture to the address the stack actually serves on, or
leave it as a deliberate test of the override and assert on it. This is the one change that could turn a green
suite red for a reason unrelated to the code being tested.

**`docs/usage.md` documents these as paths.** If links become absolute in the UI, that page should say what the
URLs look like now.

## Not in scope

- The `tangle-cli` command preview, which takes `--server-address` separately and has its own story.
- Any redirect or canonical-host enforcement on the server. This is about what a link *says*, not about
  rejecting requests that arrive by another name.

## What shipped

All three stages, with the recommended answer to each open decision. The decision itself is recorded in
[ADR 0027](../../adrs/0027-serve-the-public-domain-at-runtime-for-copyable-links.md).

**Server.** `normalizeDomain` in `internal/tangle/loader.go` trims whitespace, requires an `http`/`https` scheme
and a host, drops a trailing slash and keeps any sub-path. A malformed value stops startup with a message that
names it — verified by hand: `TANGLE_DOMAIN=not-a-url` exits with
`invalid domain "not-a-url": needs an http:// or https:// scheme`. `GET /api/config` serves
`{"domain": "..."}`, documented in swagger, and `domain` now has an entry and a section in
`docs/configuration.md` where it previously had neither.

**Frontend.** `$lib/backend/config.ts` fetches the endpoint and never rejects — a non-OK response, a thrown
request or an unexpected body all yield an empty domain, so an older server simply falls back. The root
`+layout.ts` awaits it, rather than streaming: the copyable links need a base at first render, and a path that
turns into a URL a moment later is worse than a few milliseconds on a same-origin request. `$lib/ui/links.ts`
holds the one decision — configured domain, else `window.location.origin`, else the path unchanged — and
`QueryPreview` is the only caller.

**The `integration/tangle.yaml` trap was real and is handled.** Its `domain` pointed at
`https://tangle.test.com`, which resolves nowhere; it now points at `http://localhost:8081`, where that stack
actually serves. A comment in the file says why, so it doesn't get "tidied" back.

### Decisions, as recommended

- A configured domain always wins when set. Predictable beats clever, and controlling the output is the reason
  to set it.
- A malformed domain fails at startup. The loader's comment contrasts this with the unknown-`TANGLE_`-variable
  handling from [#234](https://github.com/ivanklee86/tangle/issues/234), which ignores rather than rejects —
  those aren't ours to validate, Kubernetes injects them; this one is a typo in configuration the operator owns.
- `/api/config` carries only `domain`, but is named for the general case so version or instance metadata has
  somewhere to go.

### Coverage

`TestLoadConfigDomain` runs the real loading path over a written config file rather than calling the unexported
helper, covering six valid shapes and four rejected ones, and asserting the error names the offending value.
`TestConfigHandler` covers both the configured and unset cases, the latter pinning that the field is present
and empty rather than omitted — a missing key would be indistinguishable from an older server, which the client
treats differently. On the frontend, `links.spec.ts` and `config.spec.ts` cover precedence, normalisation and
all three degradation paths; the mocked e2e proves the link is a full URL, that a served domain overrides the
browser origin, and that the page still renders when `/api/config` 404s; and a live e2e does the real round
trip — server reads the YAML, serves it, browser builds the link on it.

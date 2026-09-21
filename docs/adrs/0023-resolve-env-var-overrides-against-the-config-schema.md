---
status: "accepted"
date: 2026-09-21
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Resolve `TANGLE_` environment variable overrides against the config schema

## Context and Problem Statement

`tangle-server` crash-looped in the `tools-testing` namespace with:

```text
Cannot load configuraiton. Error: decoding failed due to the following error(s):
'port' expected type 'int', got unconvertible type 'map[string]interface {}'
```

The loader (`internal/tangle/loader.go`) fed *every* `TANGLE_`-prefixed environment variable into
koanf, lowercasing the name and turning `_` into koanf's `.` delimiter. Kubernetes, meanwhile,
injects Docker-link-style variables into every pod that shares a namespace with a Service, named
after the Service, unless the pod sets `enableServiceLinks: false` (which defaults to `true`). The
Service is named `tangle`, so the pod's environment contained:

```text
TANGLE_SERVICE_HOST=10.43.1.2
TANGLE_SERVICE_PORT=8080
TANGLE_PORT=tcp://10.43.1.2:8080
TANGLE_PORT_8080_TCP=tcp://10.43.1.2:8080
TANGLE_PORT_8080_TCP_PROTO=tcp
TANGLE_PORT_8080_TCP_PORT=8080
TANGLE_PORT_8080_TCP_ADDR=10.43.1.2
```

`TANGLE_PORT_8080_TCP_ADDR` became the key `port.8080.tcp.addr`, which koanf unflattened into a map
at `port`, where `TangleConfig.Port` is an `int`. The app's own config prefix and the injected
Service link prefix are the same string, and nothing in the loader distinguished them.

Two properties of the collision matter for the fix. Most of the injected variables (`port.8080.*`,
`service.host`) name keys that don't exist in `TangleConfig` at all. But `TANGLE_PORT` names a key
that very much does exist — so no name-based rule alone can tell the injected `tcp://10.43.1.2:8080`
apart from an operator legitimately setting `TANGLE_PORT=9090`.

Worth noting what the `_` → `.` transform was buying: nothing that's documented. `docs/configuration.md`
documents overrides only as flat `TANGLE_<var>`, and because the transform lowercases the whole name,
a camelCase key like `sortOrder` was never reachable as `TANGLE_SORT_ORDER` anyway (that becomes
`sort.order`). The transform's only real use is reaching into the `argocds` map
(`TANGLE_ARGOCDS_TEST_ADDRESS`), which is undocumented but works today.

## Decision Drivers

- The immediate failure is a crash loop in a deployed environment; the fix has to actually resolve
  *that* environment, not just the tidier half of it.
- Silently decoding arbitrary same-prefixed variables into config is the root defect. A fix that
  only special-cases the Kubernetes variables leaves the next prefix collision to be rediscovered
  the same way.
- Whatever ignores a variable must be *visible*. This failure cost an investigation precisely
  because a variable nobody set became config with no trace.
- Existing behavior that works (`TANGLE_NAME`, `TANGLE_ARGOCDS_TEST_ADDRESS`) must keep working;
  this is a bug fix, not a config-interface change.

## Considered Options

- **Set `enableServiceLinks: false` on the Deployment only** — the narrowest fix, and the only one
  that removes the injected variables at the source.
- **Resolve env var names against `TangleConfig`'s schema, plus reject the injected value shape**
  (chosen) — ignore any `TANGLE_` variable that doesn't name a real config key, and any whose value
  has the `tcp://host:port` shape Kubernetes writes into `<SERVICE>_PORT`.
- **Drop the `_` → `.` transform** — ignore nesting entirely, so `port_8080_tcp_addr` is one flat
  unknown key.
- **Rename the config prefix** (e.g. `TANGLE_CONFIG_`) so it cannot collide with a Service named
  `tangle`.

## Decision Outcome

Chosen option: **resolve env var names against the config schema, plus reject the injected value
shape**, implemented in `internal/tangle/envkeys.go` and wired into the loader's `TransformFunc`.

1. `TangleConfig` is reflected once at startup into a tree of its `koanf` tags. An env var's
   dot-delimited key is walked against that tree; anything that doesn't land on a leaf is ignored.
   That covers `port.8080.tcp.addr` (no `8080` under the `port` leaf), `service.host` (no `service`
   key), and `argocds` on its own (a map, not a value). Keys under the `argocds` map pass through
   as written, since they're user-chosen instance names.
2. A value matching `^(tcp|udp|sctp)://` is ignored regardless of its key. This is the only rule
   that catches `TANGLE_PORT=tcp://10.43.1.2:8080`, and it's tight: those three schemes are exactly
   what kubelet writes (lowercased from a Service port's `protocol`), and no Tangle config value is
   a URL of that form. `TANGLE_PORT=9090` is unaffected, and covered by a test.
3. Resolution returns the tag's own spelling, so `TANGLE_LISTWORKERS` produces the key `listWorkers`
   rather than `listworkers`. Previously an env override and a file value for the same camelCase
   setting were two distinct koanf keys that happened to reconcile in `mapstructure`'s
   case-insensitive field matching; now they're one key and the override genuinely overrides.
4. Ignored variable names are collected onto `TangleConfig.IgnoredEnvVars` and logged once at `WARN`
   by `New`, naming each one. In the reported environment this turns a fatal decode error into a
   startup line that says exactly which variables were dropped.

`enableServiceLinks: false` is **also** worth setting on the chart's Deployment in
`ivanklee86/tangle-deployments` — it removes eight useless variables from the pod and is defense in
depth for this whole class of collision. It is not done here (different repository), and this ADR
deliberately does not depend on it: the code fix alone is sufficient, which matters because the
chart is consumed by users who may be running an older revision of it.

### Consequences

- Good, because the deployed crash loop is resolved by the binary itself, on any chart revision.
- Good, because *every* future `TANGLE_`-prefixed collision — not just Kubernetes' — is now ignored
  and named in the logs rather than silently decoded.
- Good, because env overrides of camelCase keys now collapse onto the file's key instead of relying
  on `mapstructure`'s case-insensitive matching to reconcile two keys.
- Neutral, because the schema is reflected at package init; `TangleConfig` is small and this happens
  once per process.
- Bad, because a typo'd variable (`TANGLE_TIMEOUTT=30`) is now silently ignored where it was
  previously silently ignored *by a different mechanism* (decoded into an unused key). The `WARN`
  line is what makes this better than before, not the ignoring itself — but it's a `WARN`, not a
  startup failure, because failing hard on an unrecognized `TANGLE_` variable would reintroduce
  exactly the crash loop this ADR fixes the moment a Service named `tangle` exists.
- Bad, because the value-shape rule is a heuristic living next to a schema-driven rule. It's
  documented and narrow, but it is a second kind of thing, and a config value that legitimately
  looked like `tcp://…` would be dropped.

## Pros and Cons of the Options

### `enableServiceLinks: false` alone

- Good, because it removes the injected variables at the source, and is one line in the chart.
- Bad, because it lives in a different repository, so a user on an older chart revision still
  crash-loops against a current binary.
- Bad, because it fixes this collision and no other — the loader would still decode any
  `TANGLE_`-prefixed variable anything else in the environment happens to set.

### Schema resolution + value shape (chosen)

- Good, because it fixes the whole class in the binary, and the ignored variables become visible.
- Bad, because catching `TANGLE_PORT` needs the value-shape heuristic; the schema alone can't
  distinguish it from a real override.

### Drop the `_` → `.` transform

- Good, because it's the smallest possible diff and removes a transform that buys nothing for any
  documented key.
- Bad, because it does not fix the reported failure: `TANGLE_PORT` is still `port`, and the error
  merely changes from `got map[string]interface {}` to `got string`.
- Bad, because it silently removes the undocumented but working `TANGLE_ARGOCDS_<name>_<field>`
  overrides.

### Rename the config prefix

- Good, because a prefix that isn't the Service name cannot collide with a Service link.
- Bad, because it's a breaking change to a documented interface for every existing deployment, in
  exchange for solving one instance of a general problem — any prefix collides with *something*.

## More Information

- Kubernetes Service link variables and `enableServiceLinks` —
  <https://kubernetes.io/docs/concepts/services-networking/service/#environment-variables>
- koanf's env provider ignores a variable when `TransformFunc` returns an empty key —
  `github.com/knadh/koanf/providers/env/v2@v2.0.1`
- Implementation: `internal/tangle/envkeys.go`, `internal/tangle/loader.go`; regression coverage in
  `internal/tangle/loader_test.go` (`TestEnvironmentVariables`)
- Follow-up, not addressed here: `docs/configuration.md` documents the parallelism setting as
  `manifestWorkers`, but the struct tag is `manifestsWorkers` — the documented spelling silently
  does nothing. `integration/tangle.yaml` has the same typo.

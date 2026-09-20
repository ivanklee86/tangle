# Node.js 22 → 24 upgrade

Status: proposed · 2026-09-19

Move the repo's three Node.js version pins from 22 ("Jod") to 24 ("Krypton"), the current Active LTS line (confirmed via `nodejs.org/dist/index.json`: `v24.21.0`, `lts: "Krypton"`, released 2026-09-07; `v22` is now in Maintenance LTS). This is a routine LTS bump, not a coupled/architectural change — no ADR, one small plan.

## Current → target

| File | Current | Target |
| --- | --- | --- |
| `Dockerfile:13` | `FROM node:22-alpine AS node` | `FROM node:24-alpine AS node` |
| `.devcontainer/Dockerfile:1` | `ARG NODE_VERSION=22` | `ARG NODE_VERSION=24` |
| `.github/workflows/ci.yaml:72` (`ts` job, `actions/setup-node@v4`) | `node-version: '22'` | `node-version: '24'` |
| `web/package.json` | no `engines` field | add `"engines": { "node": ">=24" }` |

These are the only three Node version references in the repo — confirmed by search: no `.nvmrc`/`.node-version` file, no Node feature/version in `.devcontainer/devcontainer.json` (it only points at `.devcontainer/Dockerfile`), and `web/package.json` currently has no `engines` field at all despite `web/.npmrc` already setting `engine-strict=true` — that setting is a no-op today since there's nothing for it to enforce.

**Compatibility check** (via `npm view <pkg> engines`, done during planning): `vite@6.2.6` (`^18 || ^20 || >=22`), `vitest@3.x` (`^18 || ^20 || >=22`), `@sveltejs/kit@2.70.3` (`>=18.13`), `svelte@5.57.1` (`>=18`), `eslint@9.x` (`^18.18 || ^20.9 || >=21.1`), `typescript@5.x` (`>=14.17`), `@playwright/test@1.63.0` (`>=20`) — every devDependency in `web/package.json` already accepts Node 24 with its current pinned version (no coupled dependency bump needed just to support this). `node:24-alpine` and `node:24` are published on Docker Hub (confirmed via `docker manifest inspect`).

## Steps

1. Bump the three references in the table above.
2. Add the `engines` field to `web/package.json` — this makes the already-present `engine-strict=true` in `web/.npmrc` actually do something (fail `npm install`/`npm ci` fast on the wrong Node major), which is worth doing now that there's a version worth enforcing rather than leaving it a silent no-op.
3. `task devcontainer` (builds `.devcontainer/Dockerfile`) to confirm the devcontainer image still builds with Node 24.
4. Inside a container built from the new image (or any Node 24 environment): `cd web && npm ci` (confirms the new `engines` pin doesn't reject the environment it's meant to run in) then the full existing verification pass — `npm run check && npm run lint && npm run test:unit -- --run && npm run build` — plus, if [the e2e plan](svelte-e2e-testing.md) has landed by the time this is implemented, `task ts:test:e2e` too.
5. `docker build -f Dockerfile .` (or `task docker:build`) from the repo root to confirm the production image's web-asset stage still builds cleanly under `node:24-alpine`.
6. Confirm CI's `ts` job (`task ts:install`, `task ts:lint`, `task ts:build`) passes on `actions/setup-node@v4` with `node-version: '24'`.

## Rollback

Revert the four changed lines/fields; nothing else in the repo depends on the Node major version, so this is a fully independent, trivially revertible change.

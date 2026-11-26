---
status: "accepted"
date: 2026-11-26
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Import `src/lib` through `#lib` subpath imports that name the file's extension

## Context and Problem Statement

[ADR 0029](0029-adopt-sveltekit-3.md) moved the frontend to SvelteKit 3 but kept the removed `$lib` alias alive through `alias: { $lib: 'src/lib' }` in `web/vite.config.ts`. That option is deprecated: every build printed `config_option_deprecated_alias`, and a later Kit release will remove it and break the build ([#258](https://github.com/ivanklee86/tangle/issues/258)). Kit 3's replacement is `#lib`, a Node subpath import declared in `package.json`'s `imports` field.

During the upgrade, `"#lib/*": "./src/lib/*"` with the existing extensionless imports (`#lib/ui/query`) built under Vite but failed `npm run check`. That's still true: TypeScript resolves a subpath import to the exact target path and doesn't try `.ts`, `.svelte` or `/index.ts` behind it, so `#lib/ui/query` is `Cannot find module`. Which `imports` shape, and which import spelling, works for Vite, svelte-check, ESLint, Vitest and Playwright at once?

## Decision Drivers

- Nothing deprecated left in the build, so a Kit minor can't break it.
- Follow the upstream-blessed shape, so future `sv` migrations and Kit docs apply as written.
- One import spelling that every tool resolves the same way, with no per-tool alias config.

## Considered Options

- The `sv` scaffold's `imports` with extension-bearing imports (`#lib/ui/query.ts`).
- Per-extension `imports` entries that keep imports extensionless.
- Keep `alias: { $lib: 'src/lib' }`.

## Decision Outcome

Chosen option: "The `sv` scaffold's `imports` with extension-bearing imports". `web/package.json` declares exactly what `npx sv create` scaffolds for a Kit 3 app (sv 1.7.1):

```json
"imports": {
	"#lib": "./src/lib/index.js",
	"#lib/*": "./src/lib/*"
}
```

Every import names its target file: `#lib/ui/query.ts`, `#lib/ui/components/index.ts`, `#lib/ui/components/RevisionPill.svelte`. The rewrite is the one sv 1.7.1's `sveltekit-3` `lib-alias` migration task performs: `$lib/x` becomes `#lib/x` plus whichever of `.ts`, `.js`, `/index.ts`, `/index.js` exists, and imports that already had an extension keep it. TypeScript accepts the `.ts` extensions because the generated `$app/tsconfig` sets `allowImportingTsExtensions`. `vi.mock` paths are rewritten the same way. The `alias` option is removed from `vite.config.ts`.

### Consequences

- Good, because `vite build` no longer prints `config_option_deprecated_alias`, and nothing in the build depends on a deprecated option.
- Good, because `#lib` is plain Node resolution: Vite, Vitest and TypeScript (svelte-check) read `package.json` `imports` themselves, with no alias in any tool's config. The Playwright specs under `web/e2e` don't import from `src/lib`; they treat the built app as a black box.
- Bad, because imports are longer and spell out `.ts`/`/index.ts`. A file renamed from `foo.ts` to `foo/index.ts` means updating its importers; `npm run check` fails until they are.
- Neutral, because `web/tsconfig.json` must not turn on `rewriteRelativeImportExtensions`, which reports TS2877 for `.ts` subpath imports. The sv migration flags the same thing.

### Confirmation

- `grep -rn '\$lib' web/src web/e2e` finds nothing, and `web/vite.config.ts` has no `alias`.
- `npm run check` reports 0 errors. Changing any import back to extensionless (`#lib/ui/query`) makes it fail with `Cannot find module`.
- CI's `ts` job (`ts:test:unit`, `ts:test:e2e`, `ts:lint`, `ts:build`), the `visual` job, and the `image` and `e2e` jobs building the production image.

## Pros and Cons of the Options

### The `sv` scaffold's `imports` with extension-bearing imports

- Good, because it's the shape Kit's own tooling generates and migrates to.
- Good, because the rewrite is mechanical and reproducible.
- Bad, because import paths carry extensions.

### Per-extension `imports` entries that keep imports extensionless

For example `"#lib/*.svelte": "./src/lib/*.svelte"` plus `"#lib/*": ["./src/lib/*.ts", "./src/lib/*/index.ts"]`.

- Good, because import lines wouldn't change beyond `$` → `#`.
- Bad, because it was tried in [ADR 0029](0029-adopt-sveltekit-3.md)'s investigation: it broke `vite build`, and svelte-check still failed. Node ignores array fallbacks for missing files, so the mapping depends on each tool's quirks.

### Keep `alias: { $lib: 'src/lib' }`

- Good, because nothing changes.
- Bad, because it's deprecated, warns on every build, and will break when Kit removes it.

## More Information

Replaces the stopgap in [ADR 0029](0029-adopt-sveltekit-3.md). The migration task is in the `sv` package (`src/migrate/migrations/sveltekit-3/tasks/lib-alias.ts`); `npx sv migrate sveltekit-3` runs it alongside Kit 3's other migration tasks, most of which this repo already did by hand in ADR 0029.

#!/usr/bin/env bash
# Runs playwright.visual.config.ts (screenshot tests and documentation
# screenshots) inside the official Playwright image for our pinned version,
# the one environment the baselines are valid in. Arguments go to
# `playwright test`, e.g. `--project=visual --update-snapshots`.
#
# The repo is bind-mounted and the container runs as you, so baselines, docs
# images, test results and the production build (web/build, as `npm run
# build` would write it) land in your checkout owned by you. node_modules and
# .svelte-kit are container-only tmpfs mounts: a container `npm ci` must not
# replace the host's node_modules, and `svelte-kit sync` must not rewrite the
# .svelte-kit/generated files a running `vite dev` reads.
set -euo pipefail

web_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_dir="$(cd "${web_dir}/.." && pwd)"

# Pinned to the Playwright in package-lock.json, which picks the browsers;
# the image must carry the same build of them.
playwright_version="$(node -p "require('${web_dir}/node_modules/playwright/package.json').version" 2>/dev/null || true)"
if [[ -z "${playwright_version}" ]]; then
	playwright_version="$(node -p "require('${web_dir}/package-lock.json').packages['node_modules/playwright'].version")"
fi
image="mcr.microsoft.com/playwright:v${playwright_version}-noble"

uid="$(id -u)"
gid="$(id -g)"
tmpfs_opts="exec,uid=${uid},gid=${gid},mode=0755"

# Create the tmpfs mount points first, or Docker creates them as root in
# your checkout.
mkdir -p "${web_dir}/node_modules" "${web_dir}/.svelte-kit"

tty_flag=()
if [[ -t 1 ]]; then tty_flag=(-t); fi

exec docker run --rm "${tty_flag[@]}" --ipc=host \
	--user "${uid}:${gid}" \
	-e HOME=/tmp -e CI=1 -e npm_config_cache=/tmp/.npm \
	-v "${repo_dir}:/work" -w /work/web \
	--tmpfs "/work/web/node_modules:${tmpfs_opts},size=3g" \
	--tmpfs "/work/web/.svelte-kit:${tmpfs_opts}" \
	"${image}" \
	bash -c 'npm ci --no-audit --no-fund --loglevel=error && npx playwright test -c playwright.visual.config.ts "$@"' -- "$@"

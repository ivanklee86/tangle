// Regenerates the three web-UI screenshots embedded in the root README.md
// (TangleHome.png, TangleApplications.png, TangleDiffs.png) from the actual
// running app, so they stay honest as the UI changes. TangleCLI.png is a
// terminal screenshot of `tangle-cli` and is out of scope for this script.
//
// Precondition: TangleHome.png needs nothing but the dev server itself — the
// home page's forms don't fetch anything until submitted. TangleApplications
// .png and TangleDiffs.png need a real backend already reachable at the URL
// in `.env.development` (PUBLIC_BASE_URL, defaults to http://localhost:8081)
// with actual ArgoCD data behind it — this script does not stand up ArgoCD
// itself. In this devcontainer that's `task services` (once) plus
// `tangle-server` running (e.g. via `air`, see `.air.conf`). Run this from
// `web/`: `node scripts/capture-readme-screenshots.mjs`.
//
// This starts its own `vite dev` instance on an unused port so it doesn't
// collide with one you may already have running, points Playwright at it
// in dark mode (matching the existing README screenshots), and captures
// the exact URLs the README's captions describe.

import { chromium } from 'playwright';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const webDir = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const imagesDir = path.join(webDir, '..', 'docs', 'images');

const shots = [
	{
		path: '/',
		file: 'TangleHome.png',
		// Needs no backend — the home page's forms don't fetch anything until
		// submitted, so "ready" just means the Applications form has mounted.
		ready: (page) => page.getByRole('heading', { name: 'Applications', level: 2 }),
		// The home page always starts with empty label fields (unlike /diffs,
		// which can seed its form from the URL) — drive the new key/value
		// LabelsInput UI directly to get an `env:test` chip on screen.
		interact: async (page) => {
			await page.getByRole('textbox', { name: 'Labels key' }).first().fill('env');
			await page.getByRole('textbox', { name: 'Labels value' }).first().fill('test');
			await page.getByRole('button', { name: 'Add Labels' }).first().click();
			await page.getByText('env:test').first().waitFor({ timeout: 5_000 });
		}
	},
	{
		path: '/applications/?labels=bazz:buzz',
		file: 'TangleApplications.png',
		// The "Applications" heading renders unconditionally (loading, error, or
		// results), so waiting on it can't confirm real data loaded — wait for an
		// actual application row instead.
		ready: (page) => page.getByRole('table').getByRole('link').first(),
		hint: 'is the backend (tangle-server + ArgoCD, see `task services`) actually running and reachable?'
	},
	{
		path: '/diffs/?labels=env:test&targetRef=test_gitops',
		file: 'TangleDiffs.png',
		ready: (page) => page.getByRole('heading', { name: 'Status', level: 3 }),
		hint: 'is the backend (tangle-server + ArgoCD, see `task services`) actually running and reachable?'
	}
];

function startDevServer() {
	return new Promise((resolve, reject) => {
		// Spawn the local `vite` binary directly rather than via `npx` — npx
		// adds a wrapper process whose lifecycle doesn't reliably track the
		// actual vite process it starts, which left an orphaned `vite dev`
		// running (and this script hanging on its still-open stdio pipes)
		// even after `viteProcess.kill()` was called on it.
		const viteBin = path.join(webDir, 'node_modules', '.bin', 'vite');
		const vite = spawn(viteBin, ['dev', '--port', '4319'], {
			cwd: webDir,
			detached: true // own process group, so we can kill it and any children it spawns
		});
		let resolved = false;

		const onData = (data) => {
			const text = data.toString();
			process.stdout.write(`[vite] ${text}`);
			const match = text.match(/Local:\s+http:\/\/localhost:(\d+)\//);
			if (match && !resolved) {
				resolved = true;
				resolve({ process: vite, baseUrl: `http://localhost:${match[1]}` });
			}
		};

		vite.stdout.on('data', onData);
		vite.stderr.on('data', (data) => process.stderr.write(`[vite] ${data}`));
		vite.on('exit', (code) => {
			if (!resolved) reject(new Error(`vite dev exited early (code ${code})`));
		});

		setTimeout(() => {
			if (!resolved) reject(new Error('Timed out waiting for vite dev to start'));
		}, 30_000);
	});
}

function stopDevServer(viteProcess) {
	try {
		// Negative pid targets the whole process group created by `detached`.
		process.kill(-viteProcess.pid, 'SIGKILL');
	} catch {
		// Already exited — nothing to do.
	}
}

async function main() {
	const { process: viteProcess, baseUrl } = await startDevServer();

	try {
		const browser = await chromium.launch();
		const context = await browser.newContext({
			viewport: { width: 1920, height: 1080 },
			colorScheme: 'dark'
		});
		const page = await context.newPage();

		for (const shot of shots) {
			const url = `${baseUrl}${shot.path}`;
			console.log(`Capturing ${shot.file} from ${url}`);
			await page.goto(url, { waitUntil: 'networkidle' });

			try {
				await shot.ready(page).waitFor({ timeout: 15_000 });
			} catch {
				const hint = shot.hint ?? 'check the dev server output above for errors.';
				throw new Error(`Timed out waiting for real content on ${shot.path} — ${hint}`);
			}

			if (shot.interact) {
				await shot.interact(page);
			}

			const outPath = path.join(imagesDir, shot.file);
			await page.screenshot({ path: outPath, fullPage: true });
			console.log(`  saved ${path.relative(process.cwd(), outPath)}`);
		}

		await browser.close();
	} finally {
		stopDevServer(viteProcess);
	}
}

main()
	.then(() => process.exit(0))
	.catch((error) => {
		console.error(error);
		process.exit(1);
	});

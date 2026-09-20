import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import type { Page } from '@playwright/test';

function loadFixture<T>(name: string): T {
	const path = fileURLToPath(new URL(name, import.meta.url));
	return JSON.parse(readFileSync(path, 'utf-8')) as T;
}

// Deliberately not typed against $lib/backend/data: these specs treat the
// built app as a black box over HTTP, the same way a real tangle-server
// would be — importing $lib's types here would need Playwright's own
// TypeScript transform to resolve SvelteKit's $lib alias, which it isn't
// configured to do.
interface ApplicationLink {
	name: string;
	url: string;
	health: string;
	syncStatus: string;
	liveRef: string;
}

interface ApplicationsFixture {
	results: {
		name: string;
		link: string;
		applications: ApplicationLink[];
	}[];
}

interface DiffFixture {
	liveManifests: string;
	targetManifests: string;
	diffs: string;
	manifestGenerationError: string;
}

const applicationsFixture = loadFixture<ApplicationsFixture>('applications.json');
const diffFixture = loadFixture<DiffFixture>('diff.json');

// Mocks tangle-server's two API endpoints so specs exercise the real built
// app and a real browser against network-mocked responses — this repo's
// "mocked (integration-layer)" rung of the test pyramid. Set up before
// page.goto() in each spec's test.beforeEach.
async function mockTangleAPI(page: Page): Promise<void> {
	await page.route('**/api/applications*', (route) => route.fulfill({ json: applicationsFixture }));
	await page.route('**/api/argocd/*/applications/*/diffs', (route) =>
		route.fulfill({ json: diffFixture })
	);
}

export {
	applicationsFixture,
	diffFixture,
	mockTangleAPI,
	type ApplicationsFixture,
	type DiffFixture
};

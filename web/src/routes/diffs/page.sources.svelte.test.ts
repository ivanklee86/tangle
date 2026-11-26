import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '#lib/backend/data.ts';
import { emptyQuery, type Query } from '#lib/ui/query.ts';
import { EMPTY_CONFIG } from '#lib/backend/config.ts';

const goto = vi.fn();
vi.mock('$app/navigation', () => ({
	goto: (url: string) => goto(url)
}));

const VALUES = 'https://github.com/org/values.git';
const CHARTS = 'https://github.com/org/charts.git';

const FLEET: ApplicationResponseStore = {
	response: {
		results: [
			{
				name: 'test',
				link: '',
				applications: [
					{
						name: 'changed-app',
						url: '',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: '',
						sources: [
							{
								position: 1,
								kind: 'helm',
								repoURL: 'https://charts.example.com',
								chart: 'web',
								targetRevision: '1.0.0'
							},
							{ position: 2, kind: 'git', repoURL: VALUES, ref: 'values', targetRevision: 'v1.2' }
						]
					},
					{
						name: 'needs-repo-app',
						url: '',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: '',
						sources: [
							{ position: 1, kind: 'git', repoURL: CHARTS, path: 'chart', targetRevision: 'main' },
							{ position: 2, kind: 'git', repoURL: VALUES, path: 'values', targetRevision: 'main' }
						]
					},
					{ name: 'quiet-app', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
				]
			}
		]
	},
	errorResponse: { error: '' },
	error: false,
	loaded: true
};

// What the server says for each application.
const RESPONSES: Record<string, object> = {
	'changed-app': {
		status: 'ok',
		diffs: '@@ -1 +1 @@\n-a\n+b\n',
		sources: [
			{
				position: 1,
				kind: 'helm',
				repoURL: 'https://charts.example.com',
				chart: 'web',
				liveRevision: '1.0.0',
				targetRevision: '1.0.0',
				moved: false
			},
			{
				position: 2,
				kind: 'git',
				repoURL: VALUES,
				ref: 'values',
				usedBy: [1],
				liveRevision: 'v1.2',
				targetRevision: 'feature-x',
				moved: true
			}
		]
	},
	'needs-repo-app': {
		status: 'needsTargetRepo',
		manifestGenerationError: 'needs target repo: …',
		candidateRepos: ['github.com/org/charts', 'github.com/org/values']
	},
	'quiet-app': { status: 'notAffected' }
};

function stubDiffs() {
	const fetchMock = vi.fn((url: string) => {
		const application = url.split('/applications/')[1].split('/')[0];
		return Promise.resolve({
			status: 200,
			text: () =>
				Promise.resolve(
					JSON.stringify({
						liveManifests: '',
						targetManifests: '',
						diffs: '',
						manifestGenerationError: '',
						...RESPONSES[application]
					})
				)
		});
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

const QUERY: Query = { ...emptyQuery(), targetRef: 'feature-x', targetRepos: [VALUES] };

function data(query: Query = QUERY) {
	return {
		params: {},
		data: { config: EMPTY_CONFIG, query, applications: Promise.resolve(FLEET) }
	};
}

// Test cases (the Diffs page with multi-source support):
//   - every diff request carries the query's target repos and inference
//   - "not affected" is hidden until the toggle shows it, and "All" counts
//     only what's listed (D10)
//   - "needs target repo" gets its own segment and row label, not "Error"
//   - the summary counts both new outcomes
//   - the needs-repo action adds the picked repos to the URL and re-runs (D8)
//   - a multi-source diff shows which source moved, and where from (D9)
describe('diffs +page.svelte with multi-source applications', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		goto.mockClear();
	});

	test('sends the target repos and inference with every diff', async () => {
		const fetchMock = stubDiffs();
		await render(Page, data({ ...QUERY, inferTargetRepo: true }));

		await expect.poll(() => fetchMock.mock.calls.length).toBe(3);
		for (const [, init] of fetchMock.mock.calls as unknown as [string, RequestInit][]) {
			expect(JSON.parse(init.body as string)).toMatchObject({
				targetRef: 'feature-x',
				targetRepos: [VALUES],
				inferTargetRepo: true
			});
		}
	});

	test('hides "not affected" until asked, and counts only what is listed', async () => {
		stubDiffs();
		const screen = await render(Page, data());
		const nav = screen.getByRole('navigation', { name: 'Applications' });

		await expect.element(screen.getByRole('button', { name: 'All 2' })).toBeVisible();
		await expect.element(nav.getByText('quiet-app')).not.toBeInTheDocument();

		await screen.getByText('Show not affected (1)').click();

		await expect.element(screen.getByRole('button', { name: 'All 3' })).toBeVisible();
		await expect.element(nav.getByText('quiet-app')).toBeVisible();
		await expect.element(nav.getByText('Not affected')).toBeVisible();
	});

	test('calls out applications that need a target repo', async () => {
		stubDiffs();
		const screen = await render(Page, data());

		await expect.element(screen.getByRole('button', { name: 'Needs repo 1' })).toBeVisible();
		await expect.element(screen.getByRole('button', { name: 'Errors 0' })).toBeVisible();
		await expect.element(screen.getByText('1 needs target repo')).toBeVisible();
		await expect.element(screen.getByText('1 not affected', { exact: false })).toBeVisible();
	});

	test('adding a candidate repo updates the URL and re-runs', async () => {
		stubDiffs();
		const screen = await render(Page, data());

		await screen.getByRole('button', { name: /needs-repo-app/ }).click();
		await expect.element(screen.getByText('Which repo is', { exact: false })).toBeVisible();

		// The first candidate starts picked; pick the second as well.
		await screen.getByRole('checkbox', { name: /^org\/values/ }).click();
		await screen.getByRole('button', { name: 'Add 2 repos to target repos' }).click();

		const expected = new URLSearchParams();
		expected.set('targetRef', 'feature-x');
		for (const repo of [VALUES, 'github.com/org/charts', 'github.com/org/values']) {
			expected.append('targetRepo', repo);
		}
		expect(goto).toHaveBeenCalledWith(`/diffs?${expected}`);
	});

	test('shows which source moved, and where from', async () => {
		stubDiffs();
		const screen = await render(Page, data());

		await screen.getByRole('button', { name: /changed-app/ }).click();

		await expect.element(screen.getByText('2 sources → feature-x')).toBeVisible();
		await expect.element(screen.getByText('1 of 2 moved')).toBeVisible();
		await expect.element(screen.getByText('moved from', { exact: false })).toBeVisible();
	});

	test('a single-source query looks as it did', async () => {
		stubDiffs();
		const screen = await render(
			Page,
			data({ ...emptyQuery(), targetRef: 'feature-x', targetRepos: [CHARTS] })
		);

		await expect.element(screen.getByRole('button', { name: 'Changed 1' })).toBeVisible();
		await expect.element(screen.getByRole('button', { name: 'Errors 0' })).toBeVisible();
	});

	// Phase 2 polish:
	//   - a candidate repo says how many other listed applications use it, and
	//     says nothing when none do
	//   - a ref source says which source reads through it
	test('a candidate says how many other applications use it', async () => {
		stubDiffs();
		const screen = await render(Page, data({ ...emptyQuery(), targetRef: 'feature-x' }));

		await screen.getByRole('button', { name: /needs-repo-app/ }).click();
		await expect.element(screen.getByText('· used by 1 other app')).toBeVisible();
		expect(screen.container.textContent).not.toContain('used by 0');
	});

	test('a ref source says which source reads through it', async () => {
		stubDiffs();
		const screen = await render(Page, data());

		await screen.getByRole('button', { name: /changed-app/ }).click();
		await expect.element(screen.getByText('used by source 1')).toBeVisible();
	});

	// #293: a failed multi-source render names the sources the target ref was
	// sent to, and shows the Sources panel, because ArgoCD's error doesn't.
	test('a failed multi-source render says where the target ref went', async () => {
		const failed = {
			...RESPONSES['changed-app'],
			status: 'error',
			diffs: '',
			manifestGenerationError: "unable to resolve 'feature-x' to a commit SHA"
		};
		vi.stubGlobal(
			'fetch',
			vi.fn((url: string) => {
				const application = url.split('/applications/')[1].split('/')[0];
				const body = application === 'changed-app' ? failed : RESPONSES[application];
				return Promise.resolve({
					status: 200,
					text: () =>
						Promise.resolve(
							JSON.stringify({
								liveManifests: '',
								targetManifests: '',
								diffs: '',
								manifestGenerationError: '',
								...body
							})
						)
				});
			})
		);
		const screen = await render(Page, data());

		await screen.getByRole('button', { name: /changed-app/ }).click();
		await expect.element(screen.getByText("Couldn't generate manifests")).toBeVisible();
		await expect
			.element(
				screen.getByText(/was sent to\s+source 2 \(org\/values\)\. Check that it exists there\./)
			)
			.toBeVisible();
		await expect.element(screen.getByText('1 of 2 moved')).toBeVisible();
	});
});

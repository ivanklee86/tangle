import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';
import { emptyQuery, type Query } from '$lib/ui/query';
import { EMPTY_CONFIG } from '$lib/backend/config';

const goto = vi.fn();
vi.mock('$app/navigation', () => ({
	goto: (url: string) => goto(url)
}));

/** Stubs the diff POST every application in the fixture triggers. */
function mockDiffFetch(diffs = '', manifestGenerationError = '') {
	vi.stubGlobal(
		'fetch',
		vi.fn().mockResolvedValue({
			status: 200,
			text: () =>
				Promise.resolve(
					JSON.stringify({
						liveManifests: 'live: yes',
						targetManifests: 'target: yes',
						diffs,
						manifestGenerationError
					})
				)
		})
	);
}

/**
 * Stubs the diff POST with one pending response per application and target
 * ref, so a test decides when each diff lands and in what order.
 */
function controlledDiffFetch() {
	const pending = new Map<string, (diffs: string) => void>();
	vi.stubGlobal(
		'fetch',
		vi.fn((url: string, init: RequestInit) => {
			const application = url.split('/applications/')[1].split('/')[0];
			const { targetRef } = JSON.parse(init.body as string) as { targetRef: string };
			return new Promise((resolve) => {
				pending.set(`${application}@${targetRef}`, (diffs) =>
					resolve({
						status: 200,
						text: () =>
							Promise.resolve(
								JSON.stringify({
									liveManifests: '',
									targetManifests: '',
									diffs,
									manifestGenerationError: ''
								})
							)
					})
				);
			});
		})
	);

	return {
		async resolve(key: string, diffs: string) {
			await expect.poll(() => pending.has(key)).toBe(true);
			const settle = pending.get(key)!;
			pending.delete(key);
			settle(diffs);
		}
	};
}

// Told apart by line count in the sidebar: the fresh diff adds one line,
// the stale one three.
const FRESH = '@@ -1 +1,2 @@\n+fresh\n';
const STALE = '@@ -1 +1,4 @@\n+stale\n+stale\n+stale\n';

const settled = () => new Promise((resolve) => setTimeout(resolve, 50));

const FLEET: ApplicationResponseStore = {
	response: {
		results: [
			{
				name: 'test',
				link: '',
				applications: [
					{
						name: 'alpha',
						url: 'https://argocd.test/alpha',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: 'main'
					},
					{
						name: 'bravo',
						url: 'https://argocd.test/bravo',
						health: 'Degraded',
						syncStatus: 'OutOfSync',
						liveRef: 'main'
					}
				]
			}
		]
	},
	errorResponse: { error: '' },
	error: false,
	loaded: true
};

const ERRORED: ApplicationResponseStore = {
	response: { results: [] },
	errorResponse: { error: 'boom' },
	error: true,
	loaded: true
};

const QUERY: Query = { labels: 'foo:bar', excludeLabels: '', targetRef: 'release-25' };

function data(overrides: { query?: Query; applications?: Promise<ApplicationResponseStore> } = {}) {
	return {
		params: {},
		data: {
			config: EMPTY_CONFIG,
			query: overrides.query ?? QUERY,
			applications: 'applications' in overrides ? overrides.applications : Promise.resolve(FLEET)
		}
	};
}

describe('diffs +page.svelte', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		goto.mockClear();
	});

	// ADR 0008: a bare visit must not fan a diff-generation request per
	// application out to every Argo CD. Without a ref there is nothing to
	// compare against, so that counts as "no query" too.
	describe('with nothing to diff', () => {
		test('opens the editor when there is no query at all', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			await expect.element(screen.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();
		});

		test('opens the editor when labels are set but no target ref is', async () => {
			const screen = await render(
				Page,
				data({
					query: { labels: 'foo:bar', excludeLabels: '', targetRef: '' },
					applications: undefined
				})
			);

			await expect.element(screen.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();
		});

		test('requires a target ref before it will apply', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			await expect
				.element(screen.getByRole('button', { name: 'Apply and run diffs' }))
				.toBeDisabled();
		});

		// A ref with no labels is a real request — diff everything against it.
		// Refusing it would leave the editor reopening forever, the same trap
		// an empty Applications query used to hit.
		test('runs with a target ref and no labels', async () => {
			mockDiffFetch();
			const screen = await render(
				Page,
				data({ query: { labels: '', excludeLabels: '', targetRef: 'release-25' } })
			);

			await expect.element(screen.getByRole('heading', { name: 'alpha', level: 2 })).toBeVisible();
			await expect
				.element(screen.getByRole('heading', { name: 'Edit diff query' }))
				.not.toBeInTheDocument();
		});
	});

	test('shows placeholder rows while the applications are pending', async () => {
		mockDiffFetch();
		const screen = await render(Page, data({ applications: new Promise(() => {}) }));

		await expect.element(screen.getByText('Loading applications')).toBeInTheDocument();
	});

	// Closing the editor without a target ref leaves nothing in flight, so a
	// loading state would never resolve.
	test('says what is missing, not "loading", when the editor closes without a ref', async () => {
		const screen = await render(
			Page,
			data({
				query: { labels: 'foo:bar', excludeLabels: '', targetRef: '' },
				applications: undefined
			})
		);

		await screen.getByRole('button', { name: 'Discard' }).click();

		await expect.element(screen.getByText('Loading applications')).not.toBeInTheDocument();
		await expect
			.element(screen.getByText('Pick labels and a target ref to compare against.'))
			.toBeVisible();
		await screen.getByRole('button', { name: 'Edit query' }).first().click();
		await expect.element(screen.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();
	});

	// A long fan-out is only usable if each diff shows up as it lands.
	test('shows each diff as it arrives rather than after the slowest', async () => {
		const diffs = controlledDiffFetch();
		const screen = await render(Page, data());

		await diffs.resolve('alpha@release-25', '');

		await expect
			.element(screen.getByRole('button', { name: /alpha/ }))
			.toMatchTextContent('No changes');
		await expect
			.element(screen.getByRole('button', { name: /bravo/ }))
			.toMatchTextContent('pending');
	});

	// A reload started under one target ref must not land on the page after
	// the query has moved to another: rows match diffs by name, so the old
	// ref's diffs would show under the new ref's heading.
	test('drops a reload that finishes after the query changed', async () => {
		const diffs = controlledDiffFetch();
		const screen = await render(Page, data());
		await diffs.resolve('alpha@release-25', '');
		await diffs.resolve('bravo@release-25', '');

		await screen.getByRole('button', { name: 'Reload all' }).click();
		await screen.rerender({ data: data({ query: { ...QUERY, targetRef: 'release-26' } }).data });
		await diffs.resolve('alpha@release-26', FRESH);
		await diffs.resolve('bravo@release-26', '');
		await expect.element(screen.getByRole('button', { name: /^alpha/ })).toMatchTextContent('+1');

		await diffs.resolve('alpha@release-25', STALE);
		await diffs.resolve('bravo@release-25', '');
		await settled();

		await expect.element(screen.getByRole('button', { name: /^alpha/ })).toMatchTextContent('+1');
		await expect
			.element(screen.getByRole('button', { name: /^alpha/ }))
			.not.toMatchTextContent('+3');
	});

	// Same race for a single application's reload.
	test('drops a single-diff reload that finishes after the query changed', async () => {
		const diffs = controlledDiffFetch();
		const screen = await render(Page, data());
		await diffs.resolve('alpha@release-25', '');
		await diffs.resolve('bravo@release-25', '');

		await screen.getByRole('button', { name: 'Reload diff' }).click();
		await screen.rerender({ data: data({ query: { ...QUERY, targetRef: 'release-26' } }).data });
		await diffs.resolve('alpha@release-26', FRESH);
		await diffs.resolve('bravo@release-26', '');
		await expect.element(screen.getByRole('button', { name: /^alpha/ })).toMatchTextContent('+1');

		await diffs.resolve('alpha@release-25', STALE);
		await settled();

		await expect.element(screen.getByRole('button', { name: /^alpha/ })).toMatchTextContent('+1');
		await expect
			.element(screen.getByRole('button', { name: /^alpha/ }))
			.not.toMatchTextContent('+3');
	});

	test('surfaces an applications-level failure instead of an empty page', async () => {
		const screen = await render(Page, data({ applications: Promise.resolve(ERRORED) }));

		await expect.element(screen.getByText('boom')).toBeVisible();
	});

	describe('the sidebar', () => {
		test('lists every application under its Argo CD', async () => {
			mockDiffFetch();
			const screen = await render(Page, data());

			await expect.element(screen.getByRole('button', { name: /alpha/ })).toBeVisible();
			await expect.element(screen.getByRole('button', { name: /bravo/ })).toBeVisible();
			// 'test' is both the sidebar's instance heading and the breadcrumb.
			await expect.element(screen.getByText('test', { exact: true }).first()).toBeVisible();
		});

		test('filters by outcome', async () => {
			mockDiffFetch('--- live\n+++ target\n+added\n');
			const screen = await render(Page, data());

			// Both applications come back changed with this stub, so Errors
			// empties the list — the useful assertion is that the filter bites.
			await screen.getByRole('button', { name: 'Errors 0' }).click();

			await expect.element(screen.getByText('No applications match these filters.')).toBeVisible();
		});

		test('filters by name', async () => {
			mockDiffFetch();
			const screen = await render(Page, data());
			await expect.element(screen.getByRole('button', { name: /bravo/ })).toBeVisible();

			await screen.getByRole('searchbox', { name: 'Filter applications by name' }).fill('alph');

			await expect.element(screen.getByRole('button', { name: /bravo/ })).not.toBeInTheDocument();
		});

		test('selecting an application shows it in the detail pane', async () => {
			mockDiffFetch();
			const screen = await render(Page, data());

			await screen.getByRole('button', { name: /bravo/ }).click();

			await expect.element(screen.getByRole('heading', { name: 'bravo' })).toBeVisible();
		});
	});

	describe('the detail pane', () => {
		test('shows the first application without needing a click', async () => {
			mockDiffFetch();
			const screen = await render(Page, data());

			await expect.element(screen.getByRole('heading', { name: 'alpha' })).toBeVisible();
		});

		test('says so plainly when there is nothing between the two refs', async () => {
			mockDiffFetch('');
			const screen = await render(Page, data());

			await expect
				.element(screen.getByText('No differences between', { exact: false }))
				.toBeVisible();
		});

		test('offers the live manifests alongside the target ones', async () => {
			// Both are already in the response, and "what is actually deployed"
			// is usually the next question after reading a diff.
			mockDiffFetch('--- live\n+++ target\n+added\n');
			const screen = await render(Page, data());

			await expect.element(screen.getByRole('tab', { name: 'Live manifests' })).toBeVisible();
			await expect.element(screen.getByRole('tab', { name: 'Target manifests' })).toBeVisible();
		});

		test('reports a manifest-generation failure as an error, not an empty diff', async () => {
			mockDiffFetch('', 'chart not found');
			const screen = await render(Page, data());

			await expect.element(screen.getByText('chart not found')).toBeVisible();
		});

		test('links out to Argo CD', async () => {
			mockDiffFetch();
			const screen = await render(Page, data());

			await expect
				.element(screen.getByRole('link', { name: 'Open in ArgoCD' }))
				.toHaveAttribute('href', 'https://argocd.test/alpha');
		});
	});

	test('summarises the run in the header', async () => {
		mockDiffFetch('--- live\n+++ target\n+added\n');
		const screen = await render(Page, data());

		// The header counts, and the sidebar group repeats it as '2 changed of 2'.
		await expect.element(screen.getByText('2 changed', { exact: false }).first()).toBeVisible();
		// The summary line and the query chip both carry the ref.
		await expect.element(screen.getByText('release-25').first()).toBeVisible();
	});

	test('counts a single failure as an error, not "1 errors"', async () => {
		mockDiffFetch('', 'chart not found');
		const screen = await render(
			Page,
			data({
				applications: Promise.resolve({
					...FLEET,
					response: {
						results: [
							{
								...FLEET.response.results[0],
								applications: [FLEET.response.results[0].applications[0]]
							}
						]
					}
				})
			})
		);

		await expect.element(screen.getByText('1 error', { exact: false })).toBeVisible();
	});

	test('applying a new query navigates with it', async () => {
		mockDiffFetch();
		const screen = await render(Page, data());

		await screen.getByRole('button', { name: 'Edit query' }).click();
		await screen.getByRole('button', { name: 'Apply and run diffs' }).click();

		expect(goto).toHaveBeenCalledWith('/diffs?targetRef=release-25&labels=foo%3Abar');
	});
});

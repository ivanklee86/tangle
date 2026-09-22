import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';
import { emptyQuery, type Query } from '$lib/ui/query';

const invalidateAll = vi.fn();
const goto = vi.fn();

vi.mock('$app/navigation', () => ({
	invalidateAll: () => invalidateAll(),
	goto: (url: string) => goto(url)
}));

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((res) => {
		resolve = res;
	});
	return { promise, resolve };
}

function store(
	applications: { name: string; health: string; syncStatus: string }[],
	instance = 'test'
): ApplicationResponseStore {
	return {
		response: {
			results: [
				{
					name: instance,
					link: '',
					applications: applications.map((application) => ({
						...application,
						url: `https://argocd.test/applications/${application.name}`,
						liveRef: 'main'
					}))
				}
			]
		},
		errorResponse: { error: '' },
		error: false,
		loaded: true
	};
}

const FLEET = store([
	{ name: 'alpha', health: 'Healthy', syncStatus: 'Synced' },
	{ name: 'bravo', health: 'Degraded', syncStatus: 'OutOfSync' },
	{ name: 'charlie', health: 'Healthy', syncStatus: 'OutOfSync' }
]);

function data(overrides: { query?: Query; applications?: Promise<ApplicationResponseStore> } = {}) {
	return {
		params: {},
		data: {
			query: overrides.query ?? ({ ...emptyQuery(), labels: 'foo:bar' } as Query),
			applications: 'applications' in overrides ? overrides.applications : Promise.resolve(FLEET)
		}
	};
}

describe('applications +page.svelte', () => {
	afterEach(() => {
		invalidateAll.mockClear();
		goto.mockClear();
		vi.useRealTimers();
	});

	// ADR 0008: a bare nav click must not fan out across every Argo CD. The
	// drawer is now that gate — it opens instead of a separate full-page form.
	describe('with no query', () => {
		test('opens the editor rather than searching', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			await expect.element(screen.getByRole('heading', { name: 'Edit query' })).toBeVisible();
		});

		test('says so in the query bar instead of showing empty chips', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			await expect.element(screen.getByText('none yet — pick applications by label')).toBeVisible();
		});

		test('applying the editor navigates with the new query', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			const label = 'Include applications with all of these labels';
			await screen.getByRole('textbox', { name: `${label} key` }).fill('env');
			await screen.getByRole('textbox', { name: `${label} value` }).fill('prod');
			await screen.getByRole('button', { name: 'Add label' }).click();
			await screen.getByRole('button', { name: 'See applications' }).click();

			expect(goto).toHaveBeenCalledWith('/applications?labels=env%3Aprod');
		});

		// Submitting with nothing filled in means "show me everything", which
		// has to navigate somewhere the page can tell apart from a bare visit
		// — otherwise it lands back on this same state and the editor reopens,
		// with no way ever to see the whole fleet.
		test('applying with no labels navigates to a URL that runs the query', async () => {
			const screen = await render(Page, data({ query: emptyQuery(), applications: undefined }));

			await screen.getByRole('button', { name: 'See applications' }).click();

			expect(goto).toHaveBeenCalledWith('/applications?searched=true');
		});
	});

	describe('with an empty query that was submitted', () => {
		test('runs it and lists everything instead of reopening the editor', async () => {
			const screen = await render(Page, data({ query: emptyQuery() }));

			await expect.element(screen.getByText('alpha')).toBeVisible();
			await expect
				.element(screen.getByRole('heading', { name: 'Edit query' }))
				.not.toBeInTheDocument();
		});

		test('says the query has no filters rather than that none was given', async () => {
			const screen = await render(Page, data({ query: emptyQuery() }));

			await expect.element(screen.getByText('no filters — showing everything')).toBeVisible();
		});
	});

	test('shows placeholder rows while the applications are pending', async () => {
		const { promise } = deferred<ApplicationResponseStore>();

		const screen = await render(Page, data({ applications: promise }));

		await expect.element(screen.getByText('Loading applications')).toBeInTheDocument();
	});

	test('shows the query in the bar before any data arrives', async () => {
		// The header renders from the URL, so someone can read what was asked
		// for while the fetch is still in flight.
		const { promise } = deferred<ApplicationResponseStore>();

		const screen = await render(Page, data({ applications: promise }));

		await expect.element(screen.getByText('foo:bar')).toBeVisible();
	});

	// One table across every instance, rather than a tab per instance: the
	// question people arrive with is "what in my fleet needs attention".
	describe('the results table', () => {
		test('lists applications from every instance with their Argo CD', async () => {
			const screen = await render(Page, data());

			await expect.element(screen.getByText('alpha')).toBeVisible();
			await expect.element(screen.getByText('bravo')).toBeVisible();
			await expect.element(screen.getByRole('cell', { name: 'test' }).first()).toBeVisible();
		});

		test('defaults to worst-first, so the row needing attention is on top', async () => {
			const screen = await render(Page, data());

			// Asserted on the order the table text appears in rather than on a
			// row index, so the check survives a column or header change.
			const text = screen.getByRole('table').element().textContent ?? '';
			expect(text.indexOf('bravo')).toBeLessThan(text.indexOf('alpha'));
		});

		test('links each row out to Argo CD', async () => {
			const screen = await render(Page, data());

			// bravo sorts first by default (worst health), so it owns the first
			// link in the table.
			await expect
				.element(screen.getByRole('link', { name: 'Open in Argo CD' }).first())
				.toHaveAttribute('href', 'https://argocd.test/applications/bravo');
		});

		test('counts what is shown against the total', async () => {
			const screen = await render(Page, data());

			await expect.element(screen.getByText('Showing', { exact: false })).toBeVisible();
		});
	});

	describe('the facet toolbar', () => {
		test('narrows to applications needing attention', async () => {
			const screen = await render(Page, data());
			await expect.element(screen.getByText('alpha')).toBeVisible();

			await screen.getByRole('button', { name: 'Needs attention 2' }).click();

			// alpha is Healthy and Synced, so it drops out; charlie is healthy
			// but drifted, so it stays.
			await expect.element(screen.getByText('alpha')).not.toBeInTheDocument();
			await expect.element(screen.getByText('charlie')).toBeVisible();
		});

		test('filters by name as you type', async () => {
			const screen = await render(Page, data());

			await screen.getByRole('searchbox', { name: 'Filter by application name' }).fill('brav');

			await expect.element(screen.getByText('bravo')).toBeVisible();
			await expect.element(screen.getByText('alpha')).not.toBeInTheDocument();
		});

		test('offers a way back when the filters hide everything', async () => {
			const screen = await render(Page, data());

			await screen
				.getByRole('searchbox', { name: 'Filter by application name' })
				.fill('nothing-matches');

			await expect
				.element(screen.getByText('No applications match the filters in the toolbar.'))
				.toBeVisible();

			await screen.getByRole('button', { name: 'Clear filters' }).click();

			await expect.element(screen.getByText('alpha')).toBeVisible();
		});
	});

	test('explains an empty result set in terms of the query that produced it', async () => {
		const screen = await render(Page, data({ applications: Promise.resolve(store([])) }));

		await expect
			.element(screen.getByRole('heading', { name: 'No applications match this query' }))
			.toBeVisible();
	});

	test('carries the query through to the Diffs page', async () => {
		const screen = await render(Page, data());

		await expect
			.element(screen.getByRole('link', { name: 'Diff these applications' }))
			.toHaveAttribute('href', '/diffs?labels=foo%3Abar');
	});

	test('calls invalidateAll on an interval once auto-refresh is enabled', async () => {
		vi.useFakeTimers();

		const screen = await render(Page, data());
		await expect.element(screen.getByText('alpha')).toBeVisible();

		await screen.getByRole('checkbox', { name: 'Auto-refresh' }).click();
		await vi.advanceTimersByTimeAsync(10_000);

		expect(invalidateAll).toHaveBeenCalled();
	});

	test('does not refresh while auto-refresh is off', async () => {
		vi.useFakeTimers();

		const screen = await render(Page, data());
		await expect.element(screen.getByText('alpha')).toBeVisible();

		await vi.advanceTimersByTimeAsync(30_000);

		expect(invalidateAll).not.toHaveBeenCalled();
	});
});

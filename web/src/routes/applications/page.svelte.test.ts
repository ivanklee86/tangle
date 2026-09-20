import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';

const { pageState } = vi.hoisted(() => ({
	pageState: { url: new URL('http://localhost/applications/?labels=foo:bar') }
}));

vi.mock('$app/state', () => ({
	page: pageState
}));

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

const resolvedApplications: ApplicationResponseStore = {
	response: {
		results: [
			{
				name: 'test',
				link: '',
				applications: [
					{ name: 'alpha', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
				]
			}
		]
	},
	errorResponse: { error: '' },
	error: false,
	loaded: true
};

describe('applications +page.svelte', () => {
	afterEach(() => {
		invalidateAll.mockClear();
		goto.mockClear();
		vi.useRealTimers();
		pageState.url = new URL('http://localhost/applications/?labels=foo:bar');
	});

	test('shows the label/exclude-label form instead of searching by default', async () => {
		pageState.url = new URL('http://localhost/applications/');

		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await expect
			.element(screen.getByRole('heading', { name: 'Applications', level: 2 }))
			.toBeVisible();
		await expect.element(screen.getByRole('button', { name: 'See applications' })).toBeVisible();
	});

	test('searching navigates with the submitted labels', async () => {
		pageState.url = new URL('http://localhost/applications/');

		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('foo');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('bar');
		await screen.getByRole('button', { name: 'Add Labels' }).click();
		await screen.getByRole('button', { name: 'See applications' }).click();

		expect(goto).toHaveBeenCalledWith('/applications?labels=foo%3Abar&searched=true');
	});

	test('searching with both fields empty still navigates with a searched marker', async () => {
		pageState.url = new URL('http://localhost/applications/');

		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await screen.getByRole('button', { name: 'See applications' }).click();

		expect(goto).toHaveBeenCalledWith('/applications?searched=true');
	});

	test('skips the form and shows results when arriving with filters already in the URL', async () => {
		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await expect.element(screen.getByText('alpha')).toBeVisible();
	});

	test('shows a spinner while data.applications is pending', async () => {
		const { promise } = deferred<ApplicationResponseStore>();

		const screen = await render(Page, { params: {}, data: { applications: promise } });

		await expect.element(screen.getByRole('status')).toBeVisible();
	});

	test('renders the application grid once data.applications resolves', async () => {
		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await expect.element(screen.getByText('alpha')).toBeVisible();
	});

	test('calls invalidateAll on an interval once refresh is enabled', async () => {
		vi.useFakeTimers();

		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});
		await expect.element(screen.getByText('alpha')).toBeVisible();

		const refreshButton = screen.getByRole('button', { name: 'Toggle automatic refresh' });
		await refreshButton.click();

		await vi.advanceTimersByTimeAsync(10_000);

		expect(invalidateAll).toHaveBeenCalled();
	});
});

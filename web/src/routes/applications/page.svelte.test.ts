import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';

const invalidateAll = vi.fn();

vi.mock('$app/stores', async () => {
	const { writable } = await import('svelte/store');
	return {
		page: writable({ url: new URL('http://localhost/applications/') })
	};
});

vi.mock('$app/navigation', () => ({
	invalidateAll: () => invalidateAll()
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
		vi.useRealTimers();
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

		const refreshButton = screen.container.querySelector('button');
		expect(refreshButton).not.toBeNull();
		refreshButton!.click();

		await vi.advanceTimersByTimeAsync(10_000);

		expect(invalidateAll).toHaveBeenCalled();
	});
});

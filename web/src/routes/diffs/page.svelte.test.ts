import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';

vi.mock('$app/stores', async () => {
	const { writable } = await import('svelte/store');
	return {
		page: writable({ url: new URL('http://localhost/diffs/') })
	};
});

function mockDiffFetch() {
	vi.stubGlobal(
		'fetch',
		vi.fn().mockResolvedValue({
			status: 200,
			text: () =>
				Promise.resolve(
					JSON.stringify({
						liveManifests: '',
						targetManifests: '',
						diffs: '',
						manifestGenerationError: ''
					})
				)
		})
	);
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

const erroredApplications: ApplicationResponseStore = {
	response: { results: [] },
	errorResponse: { error: 'boom' },
	error: true,
	loaded: true
};

describe('diffs +page.svelte', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	test('shows a loading message while data.applications is pending', async () => {
		const screen = await render(Page, {
			params: {},
			data: { applications: new Promise<ApplicationResponseStore>(() => {}) }
		});

		await expect.element(screen.getByText('Loading Applications...')).toBeVisible();
	});

	test('shows the system error alert when applications failed to load', async () => {
		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(erroredApplications) }
		});

		await expect.element(screen.getByText('System error!')).toBeVisible();
		await expect.element(screen.getByText(/boom/)).toBeVisible();
	});

	test('fetches diffs and renders tabs once applications resolve', async () => {
		mockDiffFetch();

		const screen = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await expect.element(screen.getByText('alpha')).toBeVisible();
		await expect.element(screen.getByText('Status')).toBeVisible();
	});

	test('refetches diffs when a new data.applications arrives via client-side navigation', async () => {
		mockDiffFetch();

		const { rerender, getByText } = await render(Page, {
			params: {},
			data: { applications: Promise.resolve(resolvedApplications) }
		});

		await expect.element(getByText('alpha')).toBeVisible();

		const navigatedApplications: ApplicationResponseStore = {
			response: {
				results: [
					{
						name: 'test',
						link: '',
						applications: [
							{ name: 'beta', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
						]
					}
				]
			},
			errorResponse: { error: '' },
			error: false,
			loaded: true
		};

		await rerender({ data: { applications: Promise.resolve(navigatedApplications) } });

		await expect.element(getByText('beta')).toBeVisible();
	});
});

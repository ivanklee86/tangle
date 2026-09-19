import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ApplicationsGrid from './ApplicationsGrid.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';

const applications: ApplicationResponseStore = {
	response: {
		results: [
			{
				name: 'test',
				link: 'https://test.example.com',
				applications: [
					{
						name: 'beta',
						url: 'https://beta.example.com',
						health: 'Degraded',
						syncStatus: 'OutOfSync',
						liveRef: 'main'
					},
					{
						name: 'alpha',
						url: 'https://alpha.example.com',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: 'main'
					}
				]
			},
			{ name: 'empty', link: 'https://empty.example.com', applications: [] }
		]
	},
	errorResponse: { error: '' },
	error: false,
	loaded: true
};

describe('ApplicationsGrid', () => {
	test('renders one tab per ArgoCD with a non-empty applications list', async () => {
		const screen = await render(ApplicationsGrid, { applications });

		await expect.element(screen.getByText('test (2)')).toBeVisible();
		await expect.element(screen.getByText('alpha')).toBeVisible();
		await expect.element(screen.getByText('beta')).toBeVisible();
		expect(screen.getByText('empty', { exact: true }).elements()).toHaveLength(0);
	});

	test('sorts the table by clicking a column header', async () => {
		const screen = await render(ApplicationsGrid, { applications });
		await expect.element(screen.getByText('alpha')).toBeVisible();

		const nameHeader = screen.getByRole('button', { name: /^Applications/ });

		await nameHeader.click();
		await expect.element(screen.getByRole('button', { name: 'Applications ▲' })).toBeVisible();

		await nameHeader.click();
		await expect.element(screen.getByRole('button', { name: 'Applications ▼' })).toBeVisible();
	});

	test('shows the system error alert when the response is an error', async () => {
		const errored: ApplicationResponseStore = {
			response: { results: [] },
			errorResponse: { error: 'boom' },
			error: true,
			loaded: true
		};

		const screen = await render(ApplicationsGrid, { applications: errored });

		await expect.element(screen.getByText('System error!')).toBeVisible();
		await expect.element(screen.getByText(/boom/)).toBeVisible();
	});
});

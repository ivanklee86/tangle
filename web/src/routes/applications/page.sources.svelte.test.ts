import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { type ApplicationResponseStore } from '$lib/backend/data';
import { emptyQuery } from '$lib/ui/query';
import { EMPTY_CONFIG } from '$lib/backend/config';

vi.mock('$app/navigation', () => ({ invalidateAll: vi.fn(), goto: vi.fn() }));

const FLEET: ApplicationResponseStore = {
	response: {
		results: [
			{
				name: 'test',
				link: '',
				applications: [
					{
						name: 'single',
						url: '',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: 'main',
						sources: [
							{
								position: 1,
								kind: 'git',
								repoURL: 'https://github.com/org/app.git',
								path: 'deploy',
								targetRevision: 'main'
							}
						]
					},
					{
						name: 'listed',
						url: '',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: '',
						sources: [
							{
								position: 1,
								kind: 'git',
								repoURL: 'https://github.com/org/app.git',
								path: 'deploy',
								targetRevision: 'release-2'
							}
						]
					},
					{
						name: 'multi',
						url: '',
						health: 'Healthy',
						syncStatus: 'Synced',
						liveRef: '',
						sources: [
							{
								position: 1,
								kind: 'helm',
								repoURL: 'https://stefanprodan.github.io/podinfo',
								chart: 'podinfo',
								targetRevision: '6.17.0'
							},
							{
								position: 2,
								kind: 'git',
								repoURL: 'https://github.com/org/values.git',
								ref: 'values',
								targetRevision: 'main'
							}
						]
					}
				]
			}
		]
	},
	errorResponse: { error: '' },
	error: false,
	loaded: true
};

function data() {
	return {
		params: {},
		data: {
			config: EMPTY_CONFIG,
			query: { ...emptyQuery(), labels: 'foo:bar' },
			applications: Promise.resolve(FLEET)
		}
	};
}

// Test cases (the Live ref column):
//   - a single-source row shows its live ref, as before
//   - a one-entry spec.sources, which has no liveRef, shows its source's
//     revision
//   - a multi-source row says how many sources and their revisions, and
//     expands to list each one with its repo, chart or ref
describe('applications +page.svelte with multi-source applications', () => {
	test('shows a live ref for single-source rows', async () => {
		const screen = await render(Page, data());
		const row = (name: string) => screen.getByRole('row').filter({ hasText: name });

		await expect.element(row('single').getByText('main', { exact: true })).toBeVisible();
		await expect.element(row('listed').getByText('release-2', { exact: true })).toBeVisible();
	});

	test('expands a multi-source row to list its sources', async () => {
		const screen = await render(Page, data());
		const toggle = screen.getByRole('button', { name: '2 sources' });

		await expect.element(toggle).toHaveAttribute('aria-expanded', 'false');
		await expect.element(screen.getByText('6.17.0 · main')).toBeVisible();
		await expect.element(screen.getByText('org/values')).not.toBeInTheDocument();

		await toggle.click();

		await expect.element(toggle).toHaveAttribute('aria-expanded', 'true');
		await expect.element(screen.getByText('org/values')).toBeVisible();
		await expect.element(screen.getByText('ref: values')).toBeVisible();
		await expect.element(screen.getByText('stefanprodan.github.io/podinfo')).toBeVisible();
	});
});

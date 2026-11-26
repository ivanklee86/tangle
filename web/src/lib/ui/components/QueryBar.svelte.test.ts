import { afterEach, describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import QueryBar from './QueryBar.svelte';
import { emptyQuery } from '$lib/ui/query';
import { setConfiguredDomain } from '$lib/ui/links';

describe('QueryBar', () => {
	afterEach(() => {
		setConfiguredDomain('');
		vi.restoreAllMocks();
	});

	// ADR 0027: behind a port-forward the address bar says localhost, so the
	// copied link has to be built on the configured domain, the same as the
	// query preview's link, or the two copy controls disagree.
	test('copies the current view on the configured domain, not the address bar', async () => {
		setConfiguredDomain('https://tangle.corp');
		const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();

		const screen = await render(QueryBar, {
			title: 'Applications',
			query: { ...emptyQuery(), labels: 'env:test' },
			onEdit: () => {}
		});
		await screen.getByRole('button', { name: 'Copy link to this view' }).click();

		expect(writeText).toHaveBeenCalledWith(
			`https://tangle.corp${window.location.pathname}${window.location.search}`
		);
	});

	test('falls back to the browser origin when no domain is configured', async () => {
		const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();

		const screen = await render(QueryBar, {
			title: 'Applications',
			query: emptyQuery(),
			onEdit: () => {}
		});
		await screen.getByRole('button', { name: 'Copy link to this view' }).click();

		expect(writeText).toHaveBeenCalledWith(
			`${window.location.origin}${window.location.pathname}${window.location.search}`
		);
	});

	// More than two target repos: two shown, then "+N more", which names the
	// rest and opens the editor.
	test('shows two target repos, then how many more', async () => {
		const onEdit = vi.fn();
		const screen = await render(QueryBar, {
			title: 'Diffs',
			showTargetRef: true,
			query: {
				...emptyQuery(),
				targetRef: 'feature-x',
				targetRepos: [
					'https://github.com/org/charts.git',
					'https://github.com/org/values.git',
					'https://github.com/org/platform.git'
				]
			},
			onEdit
		});

		await expect.element(screen.getByText('org/charts')).toBeVisible();
		await expect.element(screen.getByText('org/values')).toBeVisible();
		await expect.element(screen.getByText('org/platform')).not.toBeInTheDocument();

		await screen.getByRole('button', { name: /^1 more target repos: org\/platform/ }).click();
		expect(onEdit).toHaveBeenCalled();
	});

	// The server only infers without target repos, so the badge only shows then.
	test('shows "repo inferred" only without target repos', async () => {
		const screen = await render(QueryBar, {
			title: 'Diffs',
			showTargetRef: true,
			query: { ...emptyQuery(), targetRef: 'feature-x', inferTargetRepo: true },
			onEdit: () => {}
		});
		await expect.element(screen.getByText('repo inferred')).toBeVisible();

		await screen.rerender({
			query: {
				...emptyQuery(),
				targetRef: 'feature-x',
				inferTargetRepo: true,
				targetRepos: ['https://github.com/org/values.git']
			}
		});
		await expect.element(screen.getByText('repo inferred')).not.toBeInTheDocument();
	});
});

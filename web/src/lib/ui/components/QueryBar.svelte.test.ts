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
});

import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import QueryDrawer from './QueryDrawer.svelte';
import { applicationsHref, emptyQuery, type Query } from '$lib/ui/query';

const INCLUDE = 'Include applications with all of these labels';

function props(overrides: { open?: boolean; onApply?: (query: Query) => void } = {}) {
	return {
		open: overrides.open ?? true,
		title: 'Edit query',
		description: 'Applying searches every configured ArgoCD instance.',
		query: { ...emptyQuery(), labels: 'env:test' },
		targetRefMode: 'hidden' as const,
		hrefFor: applicationsHref,
		onApply: overrides.onApply ?? (() => {})
	};
}

describe('QueryDrawer', () => {
	// Closing without applying must leave nothing behind: the next open shows
	// the page's query, both as chips and as what Apply would submit.
	test('reopening after a discard shows the page query, not the discarded draft', async () => {
		const onApply = vi.fn();
		const screen = await render(QueryDrawer, props({ onApply }));

		await screen.getByRole('textbox', { name: `${INCLUDE} key` }).fill('team');
		await screen.getByRole('textbox', { name: `${INCLUDE} value` }).fill('platform');
		await screen.getByRole('button', { name: 'Add label' }).click();
		await expect.element(screen.getByText('team:platform')).toBeVisible();

		await screen.getByRole('button', { name: 'Discard' }).click();
		await screen.rerender({ open: true });

		await expect.element(screen.getByText('env:test')).toBeVisible();
		await expect.element(screen.getByText('team:platform')).not.toBeInTheDocument();

		// Adding after the reopen builds on the page's query, not the draft.
		await screen.getByRole('textbox', { name: `${INCLUDE} key` }).fill('tier');
		await screen.getByRole('textbox', { name: `${INCLUDE} value` }).fill('web');
		await screen.getByRole('button', { name: 'Add label' }).click();
		await screen.getByRole('button', { name: 'Apply' }).click();

		expect(onApply).toHaveBeenCalledWith({
			labels: 'env:test,tier:web',
			excludeLabels: '',
			targetRef: ''
		});
	});
});

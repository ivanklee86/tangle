import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ArgoCDHealthStatus from './ArgoCDHealthStatus.svelte';

describe('ArgoCDHealthStatus', () => {
	test('shows the health text for a healthy status', async () => {
		const screen = await render(ArgoCDHealthStatus, { healthStatus: 'Healthy' });

		await expect.element(screen.getByText('Healthy')).toBeVisible();
	});

	test('shows the health text for any other status', async () => {
		const screen = await render(ArgoCDHealthStatus, { healthStatus: 'Missing' });

		await expect.element(screen.getByText('Missing')).toBeVisible();
	});
});

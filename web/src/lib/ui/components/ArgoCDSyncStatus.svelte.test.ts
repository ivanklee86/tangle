import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ArgoCDSyncStatus from './ArgoCDSyncStatus.svelte';

describe('ArgoCDSyncStatus', () => {
	test.each(['Synced', 'OutOfSync', 'Unknown'])(
		'shows the sync status text for %s',
		async (syncStatus) => {
			const screen = await render(ArgoCDSyncStatus, { syncStatus });

			await expect.element(screen.getByText(syncStatus)).toBeVisible();
		}
	);
});

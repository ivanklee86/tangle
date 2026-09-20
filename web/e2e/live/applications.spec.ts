import { expect, test } from '@playwright/test';

// Real cluster state, not a fixture — assert structurally (a row exists, a
// recognized status renders) rather than on exact application names/counts.
const KNOWN_HEALTH_STATUSES = [
	'Healthy',
	'Degraded',
	'Missing',
	'Progressing',
	'Suspended',
	'Unknown'
];
const KNOWN_SYNC_STATUSES = ['Synced', 'OutOfSync', 'Unknown'];

test.describe('applications page (live)', () => {
	test('every configured ArgoCD tab has a non-empty table with recognized statuses, sorting works, and no page errors occur', async ({
		page
	}) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/applications/?searched=true&refresh=false');

		const tabs = page.getByRole('tab');
		await expect(tabs.first()).toBeVisible();
		const tabCount = await tabs.count();
		expect(tabCount).toBeGreaterThan(0);

		for (let i = 0; i < tabCount; i++) {
			await tabs.nth(i).click();

			const rows = page.locator('tbody tr');
			await expect(rows.first()).toBeVisible();
			expect(await rows.count()).toBeGreaterThan(0);

			const firstRow = rows.first();
			const rowText = await firstRow.innerText();
			expect(KNOWN_HEALTH_STATUSES.some((status) => rowText.includes(status))).toBe(true);
			expect(KNOWN_SYNC_STATUSES.some((status) => rowText.includes(status))).toBe(true);
			await expect(firstRow.locator('svg')).toHaveCount(2);
		}

		const sortButton = page.locator('th').first().getByRole('button');
		await sortButton.click();
		await expect(page.locator('th').first()).toHaveAttribute('aria-sort', 'ascending');
		await expect(page.locator('tbody tr').first()).toBeVisible();

		expect(errors).toEqual([]);
	});
});

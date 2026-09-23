import { expect, test } from '@playwright/test';

// Real cluster state, not a fixture — assert structurally (a row exists, a
// recognized status renders) rather than on exact application names or counts.
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
	test('lists the fleet in one table with recognized statuses, sorts, and raises no page errors', async ({
		page
	}) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/applications/?labels=foo:bar');

		const rows = page.locator('tbody tr');
		await expect(rows.first()).toBeVisible({ timeout: 30_000 });
		expect(await rows.count()).toBeGreaterThan(0);

		const rowText = await rows.first().innerText();
		expect(KNOWN_HEALTH_STATUSES.some((status) => rowText.includes(status))).toBe(true);
		expect(KNOWN_SYNC_STATUSES.some((status) => rowText.includes(status))).toBe(true);

		// Every instance the server answered for shows up as a column value,
		// rather than as a tab you have to find.
		await expect(page.getByRole('link', { name: 'Open in ArgoCD' }).first()).toBeVisible();

		const sortButton = page.getByRole('button', { name: /^Application/ });
		await sortButton.click();
		await expect(page.locator('th').first()).toHaveAttribute('aria-sort', 'ascending');
		await expect(rows.first()).toBeVisible();

		expect(errors).toEqual([]);
	});

	test('the facet toolbar filters against real cluster state', async ({ page }) => {
		await page.goto('/applications/?labels=foo:bar');

		const rows = page.locator('tbody tr');
		await expect(rows.first()).toBeVisible({ timeout: 30_000 });
		const total = await rows.count();

		// Structural, not exact: whatever the cluster's health happens to be,
		// filtering to a name that cannot exist must empty the table and offer
		// a way back.
		await page
			.getByRole('searchbox', { name: 'Filter by application name' })
			.fill('no-such-application');

		await expect(page.getByText('No applications match the filters in the toolbar.')).toBeVisible();

		await page.getByRole('button', { name: 'Clear filters' }).click();
		await expect(rows).toHaveCount(total);
	});

	// ADR 0008: a bare visit must not fan a label-less search out to every
	// configured ArgoCD.
	test('a bare visit opens the query editor instead of searching', async ({ page }) => {
		await page.goto('/applications/');

		await expect(page.getByRole('heading', { name: 'Edit query', level: 2 })).toBeVisible();
	});
});

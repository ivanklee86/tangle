import { expect, test } from '@playwright/test';
import { mockTangleAPI } from '../fixtures';

test.describe('applications page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/applications/?labels=foo:bar&refresh=false');
	});

	test('renders one tab per ArgoCD with the application count', async ({ page }) => {
		await expect(page.getByRole('tab', { name: 'test (2)' })).toBeVisible();
		await expect(page.getByRole('tab', { name: 'prod (2)' })).toBeVisible();
	});

	test("switching tabs shows that ArgoCD's own table", async ({ page }) => {
		await expect(page.getByRole('cell', { name: 'frontend' })).toBeVisible();
		await expect(page.getByRole('cell', { name: 'gateway' })).not.toBeVisible();

		await page.getByRole('tab', { name: 'prod (2)' }).click();

		await expect(page.getByRole('cell', { name: 'gateway' })).toBeVisible();
		await expect(page.getByRole('cell', { name: 'frontend' })).not.toBeVisible();
	});

	test('sorting the Applications column toggles ascending then descending', async ({ page }) => {
		const sortButton = page.locator('th').first().getByRole('button');

		await expect(sortButton).toHaveText('Applications');
		// Unsorted (API order): frontend, backend.
		const rows = page.locator('tbody tr');
		await expect(rows.nth(0)).toContainText('frontend');
		await expect(rows.nth(1)).toContainText('backend');

		await sortButton.click();
		await expect(sortButton).toHaveText('Applications ▲');
		await expect(rows.nth(0)).toContainText('backend');
		await expect(rows.nth(1)).toContainText('frontend');

		await sortButton.click();
		await expect(sortButton).toHaveText('Applications ▼');
		await expect(rows.nth(0)).toContainText('frontend');
		await expect(rows.nth(1)).toContainText('backend');
	});

	test('health and sync statuses render as badges with text and an icon', async ({ page }) => {
		// Asserted on the text and the presence of an icon rather than on a
		// colour class: the colour is a theme decision (status.ts owns the
		// mapping, and its unit tests pin it), while what this page promises is
		// that a status is readable without relying on colour at all.
		const frontendRow = page.locator('tbody tr', { hasText: 'frontend' });
		await expect(frontendRow.getByText('Healthy')).toBeVisible();
		await expect(frontendRow.getByText('Synced')).toBeVisible();
		await expect(frontendRow.locator('svg')).toHaveCount(2);

		const backendRow = page.locator('tbody tr', { hasText: 'backend' });
		await expect(backendRow.getByText('Degraded')).toBeVisible();
		await expect(backendRow.getByText('OutOfSync')).toBeVisible();
		await expect(backendRow.locator('svg')).toHaveCount(2);
	});

	test('drift and breakage do not share a badge colour', async ({ page }) => {
		// OutOfSync is drift, Degraded is breakage. If they ever render the
		// same, a fleet of healthy-but-unsynced applications reads as an
		// outage — the reason the palette moved off coral in the first place.
		const outOfSync = page.locator('tbody tr', { hasText: 'backend' }).getByText('OutOfSync');
		const degraded = page.locator('tbody tr', { hasText: 'backend' }).getByText('Degraded');

		const colourOf = async (locator: ReturnType<typeof page.getByText>) =>
			locator.evaluate((el) => getComputedStyle(el.closest('span') ?? el).backgroundColor);

		expect(await colourOf(outOfSync)).not.toBe(await colourOf(degraded));
	});

	test('refresh-period select and refresh-toggle button are present and toggling changes the button color', async ({
		page
	}) => {
		await expect(page.getByRole('combobox')).toBeVisible();

		const refreshButton = page.getByRole('button', { name: 'Toggle automatic refresh' });
		await expect(refreshButton).toBeVisible();
		await expect(refreshButton).not.toHaveClass(/bg-primary/);

		await refreshButton.click();
		await expect(refreshButton).toHaveClass(/bg-primary/);
	});
});

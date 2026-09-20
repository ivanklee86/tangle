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

	test('health and sync status cells render the right icon and text', async ({ page }) => {
		const frontendRow = page.locator('tbody tr', { hasText: 'frontend' });
		await expect(frontendRow.getByText('Healthy')).toBeVisible();
		await expect(frontendRow.getByText('Synced')).toBeVisible();
		await expect(frontendRow.locator('svg.text-green-500')).toHaveCount(2);

		const backendRow = page.locator('tbody tr', { hasText: 'backend' });
		await expect(backendRow.getByText('Degraded')).toBeVisible();
		await expect(backendRow.getByText('OutOfSync')).toBeVisible();
		await expect(backendRow.locator('svg.text-red-500')).toHaveCount(2);
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

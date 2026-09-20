import { expect, test } from '@playwright/test';
import { mockTangleAPI } from '../fixtures';

test.describe('home page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/');
	});

	test('renders both cards with their labeled inputs', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'Applications', level: 2 })).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Diffs', level: 2 })).toBeVisible();

		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		await expect(
			applicationsForm.getByRole('textbox', { name: 'Labels key', exact: true })
		).toBeVisible();
		await expect(
			applicationsForm.getByRole('textbox', { name: 'Labels value', exact: true })
		).toBeVisible();
		await expect(
			applicationsForm.getByRole('textbox', { name: 'Exclude Labels key' })
		).toBeVisible();
		await expect(
			applicationsForm.getByRole('textbox', { name: 'Exclude Labels value' })
		).toBeVisible();

		const diffsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See diffs' })
		});
		await expect(diffsForm.getByRole('textbox', { name: 'Labels key', exact: true })).toBeVisible();
		await expect(
			diffsForm.getByRole('textbox', { name: 'Labels value', exact: true })
		).toBeVisible();
		await expect(diffsForm.getByPlaceholder('Git branch')).toBeVisible();
	});

	test('a malformed key keeps the Applications add-label button disabled', async ({ page }) => {
		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		const addButton = applicationsForm.getByRole('button', { name: 'Add Labels' });

		await applicationsForm
			.getByRole('textbox', { name: 'Labels key', exact: true })
			.fill('env:staging');
		await applicationsForm.getByRole('textbox', { name: 'Labels value', exact: true }).fill('prod');
		await expect(addButton).toBeDisabled();
	});

	test('a malformed key keeps the Diffs add-label button disabled', async ({ page }) => {
		const diffsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See diffs' })
		});
		const addButton = diffsForm.getByRole('button', { name: 'Add Labels' });

		await diffsForm.getByRole('textbox', { name: 'Labels key', exact: true }).fill('env:staging');
		await diffsForm.getByRole('textbox', { name: 'Labels value', exact: true }).fill('prod');
		await expect(addButton).toBeDisabled();
	});

	test('submitting Applications with valid labels navigates to /applications with the filter', async ({
		page
	}) => {
		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		await applicationsForm.getByRole('textbox', { name: 'Labels key', exact: true }).fill('env');
		await applicationsForm.getByRole('textbox', { name: 'Labels value', exact: true }).fill('prod');
		await applicationsForm.getByRole('button', { name: 'Add Labels' }).click();
		await applicationsForm.getByRole('button', { name: 'See applications' }).click();

		await page.waitForURL(
			(url) =>
				url.pathname.startsWith('/applications') && url.searchParams.get('labels') === 'env:prod'
		);
	});

	test('dark-mode toggle flips html.dark and persists across a reload', async ({ page }) => {
		// app.html hardcodes <html class="dark"> as the static default — the
		// runtime script only ever removes it (explicit 'light' preference) or
		// re-adds it (explicit 'dark' preference / system preference with no
		// stored choice yet); it never removes it as a side effect of a light
		// system preference. So the app starts dark regardless of system
		// preference, and the toggle's first click switches it *to* light.
		const html = page.locator('html');
		await expect(html).toHaveClass(/dark/);

		await page.getByRole('button', { name: 'Dark mode' }).click();
		await expect(html).not.toHaveClass(/dark/);

		await page.reload();
		await expect(html).not.toHaveClass(/dark/);
	});
});

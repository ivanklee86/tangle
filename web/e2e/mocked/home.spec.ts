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
		await expect(applicationsForm.getByPlaceholder("Labels in format 'key:value'")).toBeVisible();
		await expect(
			applicationsForm.getByPlaceholder("Labels to exclude in format 'key:value'")
		).toBeVisible();

		const diffsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See diffs' })
		});
		await expect(diffsForm.getByPlaceholder("Labels in format 'key:value'")).toBeVisible();
		await expect(diffsForm.getByPlaceholder('Git branch')).toBeVisible();
	});

	test('a malformed label keeps the Applications submit disabled', async ({ page }) => {
		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		const submit = applicationsForm.getByRole('button', { name: 'See applications' });

		await expect(submit).toBeEnabled();
		await applicationsForm.getByPlaceholder("Labels in format 'key:value'").fill('nocolon');
		await expect(submit).toBeDisabled();
	});

	test('a malformed label keeps the Diffs submit disabled', async ({ page }) => {
		const diffsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See diffs' })
		});
		const submit = diffsForm.getByRole('button', { name: 'See diffs' });

		await diffsForm.getByPlaceholder('Git branch').fill('main');
		await expect(submit).toBeEnabled();

		await diffsForm.getByPlaceholder("Labels in format 'key:value'").fill('nocolon');
		await expect(submit).toBeDisabled();
	});

	test('submitting Applications with valid labels navigates to /applications with the filter', async ({
		page
	}) => {
		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		await applicationsForm.getByPlaceholder("Labels in format 'key:value'").fill('env:prod');
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

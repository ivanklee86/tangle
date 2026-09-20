import { expect, test } from '@playwright/test';

test.describe('home page (live)', () => {
	test('loads and renders both cards with no page errors', async ({ page }) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/');

		await expect(page.getByRole('heading', { name: 'Applications', level: 2 })).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Diffs', level: 2 })).toBeVisible();

		expect(errors).toEqual([]);
	});

	test('submitting Applications with no labels navigates to /applications without error', async ({
		page
	}) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/');

		const applicationsForm = page.locator('form', {
			has: page.getByRole('button', { name: 'See applications' })
		});
		await applicationsForm.getByRole('button', { name: 'See applications' }).click();

		await page.waitForURL((url) => url.pathname.startsWith('/applications'));
		await expect(page.getByRole('heading', { name: 'Applications', level: 1 })).toBeVisible();

		expect(errors).toEqual([]);
	});

	test('dark-mode toggle works', async ({ page }) => {
		await page.goto('/');

		const html = page.locator('html');
		const startedDark = (await html.getAttribute('class'))?.includes('dark') ?? false;

		await page.getByRole('button', { name: 'Dark mode' }).click();

		if (startedDark) {
			await expect(html).not.toHaveClass(/dark/);
		} else {
			await expect(html).toHaveClass(/dark/);
		}
	});
});

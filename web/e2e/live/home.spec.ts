import { expect, test } from '@playwright/test';

const INCLUDE = 'Include applications with all of these labels';

test.describe('home page (live)', () => {
	test('renders one query editor with both actions and no page errors', async ({ page }) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/');

		await expect(
			page.getByRole('heading', { name: 'Pick applications by label', level: 1 })
		).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${INCLUDE} key` })).toBeVisible();
		await expect(page.getByRole('button', { name: 'See applications' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'See diffs' })).toBeVisible();

		expect(errors).toEqual([]);
	});

	test('submitting with no labels navigates to /applications without error', async ({ page }) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/');
		await page.getByRole('button', { name: 'See applications' }).click();

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

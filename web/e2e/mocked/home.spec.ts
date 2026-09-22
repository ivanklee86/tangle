import { expect, test } from '@playwright/test';
import { mockTangleAPI } from '../fixtures';

const INCLUDE = 'Include applications with all of these labels';
const EXCLUDE = 'Exclude applications with any of these labels';

test.describe('home page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/');
	});

	// One editor, two actions — replacing the two cards that each carried
	// their own copy of the label inputs. The query is the same thing whether
	// you list applications or diff them, and duplicating it meant retyping to
	// switch intent.
	test('renders one query editor with both actions', async ({ page }) => {
		await expect(
			page.getByRole('heading', { name: 'Pick applications by label', level: 1 })
		).toBeVisible();

		await expect(page.getByRole('textbox', { name: `${INCLUDE} key` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${INCLUDE} value` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${EXCLUDE} key` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${EXCLUDE} value` })).toBeVisible();
		await expect(page.getByPlaceholder('branch, tag or commit')).toBeVisible();

		await expect(page.getByRole('button', { name: 'See applications' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'See diffs' })).toBeVisible();
	});

	test('a malformed key keeps the add-label button disabled', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env:staging');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');

		await expect(page.getByRole('button', { name: 'Add label' })).toBeDisabled();
	});

	// Each key may appear at most once, because the API builds one Kubernetes
	// selector from them and rejects a repeated key with a 400.
	test('a key that is already a chip keeps the add-label button disabled', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('staging');

		await expect(page.getByRole('button', { name: 'Add label' })).toBeDisabled();
		await expect(page.getByText('"env" is already used')).toBeVisible();
	});

	// Diffs re-render every matching application against the ref, so there is
	// nothing to run without one.
	test('See diffs stays disabled until a target ref is entered', async ({ page }) => {
		const seeDiffs = page.getByRole('button', { name: 'See diffs' });
		await expect(seeDiffs).toBeDisabled();
		await expect(page.getByText('Enter a target ref to enable diffs')).toBeVisible();

		await page.getByPlaceholder('branch, tag or commit').fill('release-25');

		await expect(seeDiffs).toBeEnabled();
	});

	test('the link preview reflects the query as it is built', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		await expect(page.getByText('/applications?labels=env%3Aprod')).toBeVisible();
	});

	test('submitting navigates to /applications with the filter', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();
		await page.getByRole('button', { name: 'See applications' }).click();

		await page.waitForURL(
			(url) =>
				url.pathname.startsWith('/applications') && url.searchParams.get('labels') === 'env:prod'
		);
	});

	// "Show me everything" is a legitimate query. The whole round trip has to
	// work: submit an empty form, land on a URL the page runs, see the fleet —
	// not bounce back into the editor.
	test('submitting an empty form shows every application', async ({ page }) => {
		await page.getByRole('button', { name: 'See applications' }).click();

		await page.waitForURL((url) => url.pathname.startsWith('/applications'));

		await expect(page.locator('tbody tr').first()).toBeVisible();
		await expect(page.getByText('no filters — showing everything')).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Edit query', level: 2 })).toBeHidden();
	});

	test('See diffs navigates to /diffs carrying the target ref', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();
		await page.getByPlaceholder('branch, tag or commit').fill('release-25');
		await page.getByRole('button', { name: 'See diffs' }).click();

		await page.waitForURL(
			(url) =>
				url.pathname.startsWith('/diffs') &&
				url.searchParams.get('labels') === 'env:prod' &&
				url.searchParams.get('targetRef') === 'release-25'
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

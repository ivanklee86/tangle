import { expect, test } from '@playwright/test';
import { mockTangleAPI } from '../fixtures';

const INCLUDE = 'Include applications with all of these labels';
const EXCLUDE = 'Exclude applications with any of these labels';
const DIFF_WHY = 'Add target ref to enable diffs.';

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
			page.getByRole('heading', { name: 'Build a label query', level: 1 })
		).toBeAttached();

		await expect(page.getByRole('textbox', { name: `${INCLUDE} key` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${INCLUDE} value` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${EXCLUDE} key` })).toBeVisible();
		await expect(page.getByRole('textbox', { name: `${EXCLUDE} value` })).toBeVisible();
		await expect(page.getByPlaceholder('Branch, tag or commit')).toBeVisible();

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
		await expect(seeDiffs).toHaveAccessibleDescription(DIFF_WHY);

		await page.getByPlaceholder('Branch, tag or commit').fill('release-25');

		await expect(seeDiffs).toBeEnabled();
		await expect(seeDiffs).toHaveAccessibleDescription('');
	});

	// The reason used to sit beside the button and again under the CI command.
	// Now it is said once, on hover, where the question comes up.
	test('hovering the disabled See diffs button explains why', async ({ page }) => {
		await expect(page.getByRole('tooltip')).toBeHidden();

		await page.getByRole('button', { name: 'See diffs' }).hover({ force: true });

		await expect(page.getByRole('tooltip')).toHaveText(DIFF_WHY);
		await expect(page.getByText('only needed for diffs')).toHaveCount(0);
		await expect(page.getByText(/nothing to compare/)).toHaveCount(0);
	});

	// ADR 0003 records that a Flowbite upgrade once shipped an Input whose left
	// icon sat on top of its placeholder, and that real browser checks are what
	// caught it. Flowbite positions a `left` snippet over the field without
	// padding the input, so this measures the geometry rather than trusting a
	// class name.
	test('the target ref box does not print its text under the branch icon', async ({ page }) => {
		const input = page.getByPlaceholder('Branch, tag or commit');
		await expect(input).toBeVisible();

		// Measured, not asserted on a class name. The icon is 16px wide and sits
		// about 12px in, so text starting before its right edge lands on top of
		// it — which is what an unpadded Flowbite `left` snippet produces.
		const { padding, iconRight } = await input.evaluate((el) => {
			const field = el as HTMLInputElement;
			const icon = field.closest('div')?.querySelector('svg');
			const fieldLeft = field.getBoundingClientRect().left;
			const iconBox = icon?.getBoundingClientRect();
			return {
				padding: parseFloat(getComputedStyle(field).paddingInlineStart),
				iconRight: iconBox ? iconBox.right - fieldLeft : 0
			};
		});

		expect(iconRight).toBeGreaterThan(0);
		expect(padding).toBeGreaterThanOrEqual(iconRight);
	});

	test('the link preview reflects the query as it is built', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		await expect(page.getByText('/applications?labels=env%3Aprod')).toBeVisible();
	});

	// A link is meant to be sent to somebody, and "/applications?labels=..."
	// isn't something a colleague can open.
	test('the link preview is a full URL, not a path', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		const origin = new URL(page.url()).origin;
		await expect(page.getByText(`${origin}/applications?labels=env%3Aprod`)).toBeVisible();
	});

	// The case the browser can't know about: someone on a port-forward sees
	// localhost, and a link built from it is useless to anyone else. A server
	// that reports a domain overrides the origin.
	test('a configured domain overrides the browser origin', async ({ page }) => {
		await page.route('**/api/config', (route) =>
			route.fulfill({ json: { domain: 'https://tangle.corp' } })
		);
		await page.goto('/');

		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		await expect(
			page.getByText('https://tangle.corp/applications?labels=env%3Aprod')
		).toBeVisible();
	});

	// Older servers have no such endpoint, and nothing this config affects is
	// load-bearing enough to block rendering on.
	test('still renders when the config endpoint is unavailable', async ({ page }) => {
		await page.route('**/api/config', (route) => route.fulfill({ status: 404 }));
		await page.goto('/');

		await expect(
			page.getByRole('heading', { name: 'Build a label query', level: 1 })
		).toBeAttached();

		const origin = new URL(page.url()).origin;
		await expect(page.getByText(`${origin}/applications`)).toBeVisible();
	});

	// The link and the CI command are two ways of saying the same query, and a
	// query built here usually ends up as one or the other. Showing only one
	// per screen meant knowing in advance which screen to build it on.
	test('previews both the link and the tangle-cli command', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();

		await expect(page.getByText('/applications?labels=env%3Aprod')).toBeVisible();
		await expect(page.getByText('tangle-cli generate-manifests --label env=prod')).toBeVisible();
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
		await expect(page.getByText('No filters')).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Edit query', level: 2 })).toBeHidden();
	});

	test('See diffs navigates to /diffs carrying the target ref', async ({ page }) => {
		await page.getByRole('textbox', { name: `${INCLUDE} key` }).fill('env');
		await page.getByRole('textbox', { name: `${INCLUDE} value` }).fill('prod');
		await page.getByRole('button', { name: 'Add label' }).click();
		await page.getByPlaceholder('Branch, tag or commit').fill('release-25');
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

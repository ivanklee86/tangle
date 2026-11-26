import { expect, test } from '@playwright/test';
import {
	CHECKOUT_API,
	diffsPath,
	diffsSettled,
	openScreen,
	ready,
	PLATFORM_VALUES,
	selectDiff,
	THEMES
} from '../fixtures/screens';

// Screenshot tests (#294). Each screen is one the multi-source work changed or
// one where a visual regression slipped past the functional tests, rendered on
// the fixture fleet in both themes. Run through scripts/playwright-docker.sh:
// the baselines only hold in the image that made them.
//
// Screens:
//   - Diffs: a query with no target repo (every outcome, needs-repo detail),
//     the same with a target repo (Sources panel and diff), "not affected"
//     shown, and the manifest-generation error
//   - Applications: multi-source rows expanded
//   - the query drawer with target repos and inference
//   - Home
//   - the Diffs and Applications pages in a 1024px window

const QUERY = { labels: 'team:payments', targetRef: 'feature/queue-tuning' };

for (const theme of THEMES) {
	test.describe(`${theme} theme`, () => {
		test('diffs: needs target repo', async ({ page }) => {
			await openScreen(page, diffsPath(QUERY), theme);
			await diffsSettled(page);
			await selectDiff(page, 'payments-worker');

			await ready(page);
			await expect(page).toHaveScreenshot(`diffs-needs-repo-${theme}.png`);
		});

		test('diffs: multi-source with a target repo', async ({ page }) => {
			await openScreen(page, diffsPath({ ...QUERY, targetRepo: PLATFORM_VALUES }), theme);
			await diffsSettled(page);
			await selectDiff(page, 'podinfo-stack');

			await ready(page);
			await expect(page).toHaveScreenshot(`diffs-multi-source-${theme}.png`);
		});

		test('diffs: not affected shown', async ({ page }) => {
			await openScreen(page, diffsPath({ ...QUERY, targetRepo: CHECKOUT_API }), theme);
			await diffsSettled(page);
			await page.getByText(/Show not affected/).click();
			await selectDiff(page, 'search-indexer');

			await ready(page);
			await expect(page).toHaveScreenshot(`diffs-not-affected-${theme}.png`);
		});

		test('diffs: manifest generation error', async ({ page }) => {
			await openScreen(page, diffsPath(QUERY), theme);
			await diffsSettled(page);
			await selectDiff(page, 'ledger');

			await ready(page);
			await expect(page).toHaveScreenshot(`diffs-error-${theme}.png`);
		});

		test('applications: multi-source rows expanded', async ({ page }) => {
			await openScreen(page, '/applications?labels=team:payments', theme);
			for (const name of ['3 sources', '2 sources']) {
				await page.getByRole('button', { name }).click();
			}

			await ready(page);
			await expect(page).toHaveScreenshot(`applications-expanded-${theme}.png`);
		});

		test('query drawer with target repos', async ({ page }) => {
			await openScreen(
				page,
				diffsPath({
					...QUERY,
					targetRepo: [PLATFORM_VALUES, CHECKOUT_API],
					inferTargetRepo: 'true'
				}),
				theme
			);
			await diffsSettled(page);
			await page.getByRole('button', { name: 'Edit query' }).click();
			await expect(page.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();

			await ready(page);
			await expect(page).toHaveScreenshot(`query-drawer-${theme}.png`);
		});

		test('home', async ({ page }) => {
			await openScreen(page, '/', theme);
			await expect(page.getByText('Target repos')).toBeVisible();

			await ready(page);
			await expect(page).toHaveScreenshot(`home-${theme}.png`, { fullPage: true });
		});
	});
}

test.describe('1024px window', () => {
	test.use({ viewport: { width: 1024, height: 768 } });

	test('diffs', async ({ page }) => {
		await openScreen(page, diffsPath({ ...QUERY, targetRepo: PLATFORM_VALUES }));
		await diffsSettled(page);
		await selectDiff(page, 'podinfo-stack');

		await ready(page);
		await expect(page).toHaveScreenshot('diffs-narrow.png');
	});

	test('applications', async ({ page }) => {
		await openScreen(page, '/applications?labels=team:payments');
		await page.getByRole('button', { name: '3 sources' }).click();

		await ready(page);
		await expect(page).toHaveScreenshot('applications-narrow.png');
	});
});

import { expect, test } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import {
	diffsPath,
	diffsSettled,
	openScreen,
	ready,
	PLATFORM_VALUES,
	selectDiff
} from '../fixtures/screens';

// Writes the screenshots README.md and docs/index.md embed, from the fixture
// fleet instead of the live stack, so they show a realistic fleet and come
// out the same every time. Not a test: it saves images rather than comparing
// them. Run with `task ts:screenshots`. TangleCLI.png is a terminal
// screenshot and is made by hand.
//
// The queries here are the ones the captions in README.md and docs/index.md
// quote; change both together.

const imagesDir = fileURLToPath(new URL('../../../docs/images/', import.meta.url));

test.use({ viewport: { width: 1920, height: 1080 } });

test('TangleHome.png: a query being built on the Home page', async ({ page }) => {
	await openScreen(page, '/');
	const include = 'Include applications with all of these labels';
	await page.getByRole('textbox', { name: `${include} key` }).fill('team');
	await page.getByRole('textbox', { name: `${include} value` }).fill('payments');
	await page.getByRole('button', { name: 'Add label' }).click();
	await page.getByRole('textbox', { name: 'Target ref' }).fill('feature/queue-tuning');
	await page.getByRole('textbox', { name: 'Target repos' }).fill(PLATFORM_VALUES);
	await page.getByRole('button', { name: 'Add target repo' }).click();
	await expect(
		page.getByRole('button', { name: 'Remove target repo acme/platform-values' })
	).toBeVisible();

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleHome.png`, fullPage: true });
});

test('TangleApplications.png: /applications?labels=team:payments', async ({ page }) => {
	await openScreen(page, '/applications?labels=team:payments');
	await page.getByRole('button', { name: '3 sources' }).click();
	await expect(page.getByText('stefanprodan/podinfo')).toBeVisible();

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleApplications.png`, fullPage: true });
});

test('TangleDiffs.png: /diffs?labels=team:payments&targetRef=feature/queue-tuning&targetRepo=…/platform-values', async ({
	page
}) => {
	await openScreen(
		page,
		diffsPath({
			labels: 'team:payments',
			targetRef: 'feature/queue-tuning',
			targetRepo: PLATFORM_VALUES
		})
	);
	await diffsSettled(page);
	await selectDiff(page, 'payments-worker');

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleDiffs.png`, fullPage: true });
});

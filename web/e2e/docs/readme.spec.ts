import { expect, test } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import {
	diffsPath,
	diffsSettled,
	openScreen,
	PLATFORM_VALUES,
	ready,
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

test('TangleHome.png: the tier:backend label set on the Home page', async ({ page }) => {
	await openScreen(page, '/');
	const include = 'Include applications with all of these labels';
	await page.getByRole('textbox', { name: `${include} key` }).fill('tier');
	await page.getByRole('textbox', { name: `${include} value` }).fill('backend');
	await page.getByRole('button', { name: 'Add label' }).click();
	await expect(page.getByRole('button', { name: 'Remove label tier:backend' })).toBeVisible();

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleHome.png`, fullPage: true });
});

test('TangleApplications.png: /applications?labels=tier:backend', async ({ page }) => {
	await openScreen(page, '/applications?labels=tier:backend');
	await expect(page.getByText('billing-cron', { exact: true })).toBeVisible();

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleApplications.png`, fullPage: true });
});

test('TangleDiffs.png: /diffs?labels=tier:backend&targetRef=feature/queue-tuning', async ({
	page
}) => {
	await openScreen(page, diffsPath({ labels: 'tier:backend', targetRef: 'feature/queue-tuning' }));
	await diffsSettled(page);
	await selectDiff(page, 'checkout-api');

	await ready(page);
	await page.screenshot({ path: `${imagesDir}TangleDiffs.png`, fullPage: true });
});

test('TangleMultiSource.png: /diffs?labels=team:payments&targetRef=feature/queue-tuning&targetRepo=…/platform-values', async ({
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
	await page.screenshot({ path: `${imagesDir}TangleMultiSource.png`, fullPage: true });
});

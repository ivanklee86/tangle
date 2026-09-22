import { expect, test } from '@playwright/test';
import { mockTangleAPI } from '../fixtures';

test.describe('applications page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/applications/?labels=foo:bar');
	});

	// One table across every instance, replacing the tab-per-instance layout:
	// the question people arrive with is "what in my fleet needs attention",
	// and a tab strip answers that one instance at a time.
	test('lists every instance in one table with an Argo CD column', async ({ page }) => {
		for (const name of ['frontend', 'backend', 'gateway', 'worker']) {
			await expect(page.getByRole('cell', { name, exact: true })).toBeVisible();
		}

		await expect(page.getByRole('cell', { name: 'test', exact: true }).first()).toBeVisible();
		await expect(page.getByRole('cell', { name: 'prod', exact: true }).first()).toBeVisible();
	});

	test('summarises the fleet and the query in the header', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'Applications', level: 1 })).toBeVisible();
		await expect(page.getByText('4 applications across 2 instances')).toBeVisible();
		await expect(page.getByText('foo:bar')).toBeVisible();
	});

	test('defaults to worst-first so the degraded applications lead', async ({ page }) => {
		// allTextContents() doesn't auto-wait, so wait for the table to fill
		// before reading it — otherwise this races the streamed load and reads
		// an empty list.
		await expect(page.locator('tbody tr')).toHaveCount(4);

		const names = await page.locator('tbody tr td:first-child').allTextContents();

		expect(names.slice(0, 2).sort()).toEqual(['backend', 'worker']);
	});

	test('sorting the Application column toggles ascending then descending', async ({ page }) => {
		const sortButton = page.getByRole('button', { name: /^Application/ });

		await sortButton.click();
		await expect(sortButton).toHaveText('Application ▲');
		await expect(page.locator('tbody tr').first()).toContainText('backend');

		await sortButton.click();
		await expect(sortButton).toHaveText('Application ▼');
		await expect(page.locator('tbody tr').first()).toContainText('worker');
	});

	test('health and sync statuses render as badges with text and an icon', async ({ page }) => {
		// Asserted on the text and the presence of an icon rather than on a
		// colour class: the colour is a theme decision (status.ts owns the
		// mapping, and its unit tests pin it), while what this page promises is
		// that a status is readable without relying on colour at all.
		const frontendRow = page.locator('tbody tr', { hasText: 'frontend' });
		await expect(frontendRow.getByText('Healthy')).toBeVisible();
		await expect(frontendRow.getByText('Synced')).toBeVisible();
		// Each badge carries its own icon, so the status survives being read
		// without colour. Counted per badge rather than per row — the row also
		// holds the external-link icon on the Argo CD button.
		await expect(frontendRow.getByText('Healthy').locator('svg')).toHaveCount(1);
		await expect(frontendRow.getByText('Synced').locator('svg')).toHaveCount(1);

		const backendRow = page.locator('tbody tr', { hasText: 'backend' });
		await expect(backendRow.getByText('Degraded')).toBeVisible();
		await expect(backendRow.getByText('OutOfSync')).toBeVisible();
	});

	test('drift and breakage do not share a badge colour', async ({ page }) => {
		// OutOfSync is drift, Degraded is breakage. If they ever render the
		// same, a fleet of healthy-but-unsynced applications reads as an
		// outage — the reason the palette moved off coral in the first place.
		const row = page.locator('tbody tr', { hasText: 'backend' });

		const colourOf = async (text: string) =>
			row
				.getByText(text)
				.evaluate((el) => getComputedStyle(el.closest('span') ?? el).backgroundColor);

		expect(await colourOf('OutOfSync')).not.toBe(await colourOf('Degraded'));
	});

	test.describe('the facet toolbar', () => {
		test('narrows to applications needing attention', async ({ page }) => {
			await page.getByRole('button', { name: 'Needs attention 2' }).click();

			await expect(page.getByRole('cell', { name: 'backend', exact: true })).toBeVisible();
			await expect(page.getByRole('cell', { name: 'frontend', exact: true })).toBeHidden();
		});

		test('filters by name', async ({ page }) => {
			await page.getByRole('searchbox', { name: 'Filter by application name' }).fill('gate');

			await expect(page.getByRole('cell', { name: 'gateway', exact: true })).toBeVisible();
			await expect(page.getByRole('cell', { name: 'frontend', exact: true })).toBeHidden();
		});

		test('counts what is shown against the total', async ({ page }) => {
			await page.getByRole('searchbox', { name: 'Filter by application name' }).fill('gate');

			await expect(page.getByText('Showing 1 of 4')).toBeVisible();
		});
	});

	test('carries the query through to the Diffs page', async ({ page }) => {
		await page.getByRole('link', { name: 'Diff these applications' }).click();

		await page.waitForURL((url) => url.pathname.startsWith('/diffs'));
	});

	// ADR 0008: a bare nav click must not fan a label-less search out to every
	// Argo CD. The drawer is the gate.
	test('opens the query editor instead of searching when the URL has no query', async ({
		page
	}) => {
		let requests = 0;
		await page.route('**/api/applications*', (route) => {
			requests += 1;
			return route.fulfill({ json: { results: [] } });
		});

		await page.goto('/applications/');

		await expect(page.getByRole('heading', { name: 'Edit query', level: 2 })).toBeVisible();
		await expect(page.getByText('none yet — pick applications by label')).toBeVisible();
		expect(requests).toBe(0);
	});

	test('auto-refresh toggle and period select are present', async ({ page }) => {
		const toggle = page.getByRole('checkbox', { name: 'Auto-refresh' });
		await expect(toggle).toBeVisible();
		await expect(toggle).not.toBeChecked();

		const period = page.getByRole('combobox', { name: 'Refresh period' });
		await expect(period).toBeDisabled();

		// force: Flowbite's Toggle hides the real checkbox behind a styled
		// span (sr-only + peer), so Playwright's actionability check on the
		// input itself never passes.
		await toggle.check({ force: true });
		await expect(period).toBeEnabled();
	});
});

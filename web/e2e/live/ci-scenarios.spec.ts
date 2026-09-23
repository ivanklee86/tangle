import { expect, test, type Page } from '@playwright/test';

// The UI half of the scenarios in docs/agents/test_cases.md ("CI" and "CI
// with failure"): CI renders an Applications or Diffs link, and the user
// clicks it. The link comes from a tool outside Tangle, so these tests build
// it by hand from the served domain rather than borrowing the UI's own
// helpers. That makes the URL shape itself part of what's being checked.
//
// The "change the user pushed" is the test_gitops branch on origin (see
// cmd/tangle-cli/main_e2e_test.go for the CLI half):
//
//   - test-1 (test ArgoCD): the diff adds one Ingress.
//   - test-2 (test), test-4 (prod): unchanged.
//   - test-3 (prod): manifest generation fails in helm template.
//
// Rebasing or "fixing" test_gitops breaks these tests.

const TARGET_REF = 'test_gitops';

// A real ArgoCD manifest render per application.
const DIFFS_TIMEOUT = 60_000;

async function servedDomain(page: Page): Promise<string> {
	const response = await page.request.get('/api/config');
	expect(response.ok()).toBe(true);
	const { domain } = (await response.json()) as { domain: string };
	expect(domain).not.toBe('');
	return domain;
}

async function applicationsLink(page: Page, labels: string): Promise<string> {
	return `${await servedDomain(page)}/applications?${new URLSearchParams({ labels })}`;
}

async function diffsLink(page: Page, labels: string): Promise<string> {
	return `${await servedDomain(page)}/diffs?${new URLSearchParams({ labels, targetRef: TARGET_REF })}`;
}

function collectPageErrors(page: Page): Error[] {
	const errors: Error[] = [];
	page.on('pageerror', (error) => errors.push(error));
	return errors;
}

function applicationRow(page: Page, name: string) {
	return page.locator('tbody tr').filter({ has: page.getByRole('cell', { name, exact: true }) });
}

async function expectApplicationState(page: Page, name: string, health: string, sync: string) {
	const row = applicationRow(page, name);
	// Right after bring-up an auto-synced app can still be
	// Progressing, so give the real cluster time to settle.
	await expect(async () => {
		await page.reload();
		await expect(row).toContainText(health, { timeout: 10_000 });
		await expect(row).toContainText(sync);
	}).toPass({ timeout: DIFFS_TIMEOUT });
}

function diffsSidebar(page: Page) {
	return page.getByRole('navigation', { name: 'Applications' });
}

function diffsEntry(page: Page, name: string) {
	return diffsSidebar(page).getByRole('button', { name: new RegExp(`^${name}\\b`) });
}

async function waitForAllDiffs(page: Page, count: number) {
	await expect(page.getByText(`${count} of ${count} loaded`)).toBeVisible({
		timeout: DIFFS_TIMEOUT
	});
}

async function expectIngressDiff(page: Page) {
	await diffsEntry(page, 'test-1').click();
	await expect(page.getByRole('heading', { name: 'test-1', level: 2 })).toBeVisible();
	await expect(
		page
			.locator('code:visible')
			.filter({ hasText: /kind: Ingress/ })
			.first()
	).toBeVisible();
}

test.describe('CI scenario (live)', () => {
	const labels = 'bazz:buzz';

	test('the Applications link shows each application’s health and sync', async ({ page }) => {
		const errors = collectPageErrors(page);

		await page.goto(await applicationsLink(page, labels));

		await expect(page.locator('tbody tr')).toHaveCount(2, { timeout: 30_000 });
		await expectApplicationState(page, 'test-1', 'Healthy', 'Synced');
		// test-2 has no automated sync policy, so it stays this way.
		await expectApplicationState(page, 'test-2', 'Missing', 'OutOfSync');

		expect(errors).toEqual([]);
	});

	test('the Diffs link shows which applications the change touches', async ({ page }) => {
		const errors = collectPageErrors(page);

		await page.goto(await diffsLink(page, labels));
		await waitForAllDiffs(page, 2);

		await expect(page.getByText('1 changed', { exact: true })).toBeVisible();
		await expect(page.getByText(/^\d+ errors?$/)).toHaveCount(0);

		await expect(diffsEntry(page, 'test-1')).toContainText(/\+\d+/);
		await expect(diffsEntry(page, 'test-2')).toContainText('No changes');

		await expectIngressDiff(page);

		expect(errors).toEqual([]);
	});
});

test.describe('CI with failure scenario (live)', () => {
	const labels = 'foo:bar';

	test('the Applications link lists every application on both ArgoCDs', async ({ page }) => {
		const errors = collectPageErrors(page);

		await page.goto(await applicationsLink(page, labels));

		await expect(page.locator('tbody tr')).toHaveCount(4, { timeout: 30_000 });
		for (const name of ['test-1', 'test-2', 'test-3', 'test-4']) {
			await expect(applicationRow(page, name)).toHaveCount(1);
		}
		await expect(applicationRow(page, 'test-1')).toContainText('test');
		await expect(applicationRow(page, 'test-3')).toContainText('prod');

		expect(errors).toEqual([]);
	});

	test('the Diffs link calls out the failed application without hiding the others', async ({
		page
	}) => {
		const errors = collectPageErrors(page);

		await page.goto(await diffsLink(page, labels));
		await waitForAllDiffs(page, 4);

		await expect(page.getByText('1 changed', { exact: true })).toBeVisible();
		await expect(page.getByText('1 error', { exact: true })).toBeVisible();

		await expect(diffsEntry(page, 'test-3')).toContainText('Error');
		await diffsEntry(page, 'test-3').click();
		await expect(page.getByText("Couldn't generate manifests")).toBeVisible();
		await expect(page.getByText(/helm template/)).toBeVisible();

		// The failure doesn't blank the applications that rendered.
		await expect(diffsEntry(page, 'test-2')).toContainText('No changes');
		await expect(diffsEntry(page, 'test-4')).toContainText('No changes');
		await expectIngressDiff(page);

		// The outcome filter narrows the list to just the failure.
		await page
			.getByRole('group', { name: 'Filter by diff outcome' })
			.getByRole('button', { name: /^Errors/ })
			.click();
		await expect(diffsSidebar(page).getByRole('button')).toHaveCount(1);
		await expect(diffsEntry(page, 'test-3')).toBeVisible();

		expect(errors).toEqual([]);
	});
});

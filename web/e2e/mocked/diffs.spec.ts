import { expect, test } from '@playwright/test';
import { diffFixture, mockTangleAPI } from '../fixtures';

test.describe('diffs page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/diffs/?labels=foo:bar&targetRef=main');
	});

	test('summarises the run against the target ref', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'Diffs', level: 1 })).toBeVisible();
		// Every fixture application returns the same one-line diff.
		await expect(page.getByText('4 changed', { exact: false }).first()).toBeVisible();
		// exact, or the navbar's sr-only "Open main menu" matches first.
		await expect(page.getByText('main', { exact: true }).first()).toBeVisible();
	});

	// One list across every instance, replacing the nested ArgoCD → application
	// tab strip: with a fleet of any size the old layout buried a changed
	// application two clicks deep.
	test('lists every application in the sidebar under its Argo CD', async ({ page }) => {
		for (const name of ['frontend', 'backend', 'gateway', 'worker']) {
			await expect(page.getByRole('button', { name: new RegExp(name) })).toBeVisible();
		}
	});

	test('shows the first application without needing a click', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'frontend', level: 2 })).toBeVisible();
	});

	test('selecting another application swaps the detail pane', async ({ page }) => {
		await page.getByRole('button', { name: /backend/ }).click();

		await expect(page.getByRole('heading', { name: 'backend', level: 2 })).toBeVisible();
		await expect(page.getByRole('heading', { name: 'frontend', level: 2 })).toBeHidden();
	});

	test('the detail pane names where you are and what is being compared', async ({ page }) => {
		await expect(page.getByLabel('Diff location')).toContainText('test');
		await expect(page.getByText('main → main')).toBeVisible();
	});

	test('offers the live manifests alongside the target ones', async ({ page }) => {
		// Both come back in the same response, and "what is actually deployed"
		// is usually the next question after reading a diff.
		await expect(page.getByRole('tab', { name: /^Diff/ })).toBeVisible();
		await expect(page.getByRole('tab', { name: 'Target manifests' })).toBeVisible();
		await expect(page.getByRole('tab', { name: 'Live manifests' })).toBeVisible();

		await page.getByRole('tab', { name: 'Live manifests' }).click();
		await expect(page.getByText('color: blue').filter({ visible: true })).toBeVisible();
	});

	test('filtering the sidebar by name narrows the list', async ({ page }) => {
		await expect(page.getByRole('button', { name: /gateway/ })).toBeVisible();

		await page.getByRole('searchbox', { name: 'Filter applications by name' }).fill('front');

		await expect(page.getByRole('button', { name: /gateway/ })).toBeHidden();
		await expect(page.getByRole('button', { name: /frontend/ })).toBeVisible();
	});

	test('filtering by outcome narrows the list', async ({ page }) => {
		await page.getByRole('button', { name: 'Errors 0' }).click();

		await expect(page.getByText('No applications match these filters.')).toBeVisible();
	});

	test('the reload-diff button re-issues the mocked diff POST', async ({ page }) => {
		// The route mocked in beforeEach already served the initial page load's
		// diff fetches before this test body runs, so a counting route added
		// here wouldn't see them — reload to force a fresh fetch through it.
		let diffRequestCount = 0;
		await page.route('**/api/argocd/*/applications/*/diffs', (route) => {
			diffRequestCount += 1;
			return route.fulfill({ json: diffFixture });
		});
		await page.reload();

		await expect(page.getByRole('heading', { name: 'frontend', level: 2 })).toBeVisible();
		const countAfterLoad = diffRequestCount;
		expect(countAfterLoad).toBeGreaterThan(0);

		await page.getByRole('button', { name: 'Reload diff' }).click();

		await expect.poll(() => diffRequestCount).toBeGreaterThan(countAfterLoad);
	});

	// ADR 0008: this page fans out one diff-generation POST per application to
	// every Argo CD, so a bare visit must not start it.
	test.describe('the search gate', () => {
		test('opens the editor and fetches nothing with no query at all', async ({ page }) => {
			let requests = 0;
			await page.route('**/api/applications*', (route) => {
				requests += 1;
				return route.fulfill({ json: { results: [] } });
			});

			await page.goto('/diffs/');

			await expect(page.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();
			expect(requests).toBe(0);
		});

		test('still opens the editor when labels are set but no target ref is', async ({ page }) => {
			await page.goto('/diffs/?labels=foo:bar');

			await expect(page.getByRole('heading', { name: 'Edit diff query' })).toBeVisible();
			await expect(page.getByRole('button', { name: 'Apply and run diffs' })).toBeDisabled();
		});
	});
});

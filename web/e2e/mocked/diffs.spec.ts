import { expect, test } from '@playwright/test';
import { diffFixture, mockTangleAPI } from '../fixtures';

test.describe('diffs page', () => {
	test.beforeEach(async ({ page }) => {
		await mockTangleAPI(page);
		await page.goto('/diffs/?targetRef=main');
	});

	test('renders nested ArgoCD -> application tabs', async ({ page }) => {
		await expect(page.getByRole('tab', { name: 'test', exact: true })).toBeVisible();
		await expect(page.getByRole('tab', { name: 'prod', exact: true })).toBeVisible();
		await expect(page.getByRole('tab', { name: 'frontend' })).toBeVisible();
	});

	test('an unhealthy application shows the rose alert icon on its inner tab title', async ({
		page
	}) => {
		const healthyTab = page.getByRole('tab', { name: 'frontend' });
		await expect(healthyTab.locator('svg.text-rose-500')).toHaveCount(0);

		const unhealthyTab = page.getByRole('tab', { name: 'backend' });
		await expect(unhealthyTab.locator('svg.text-rose-500')).toHaveCount(1);
	});

	test('the Status section renders health and sync text', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'Status', level: 3 })).toBeVisible();
		await expect(page.getByText('Healthy')).toBeVisible();
		await expect(page.getByText('Synced')).toBeVisible();
	});

	test('the (More Info) link points at the fixture application URL', async ({ page }) => {
		const link = page.getByRole('link', { name: 'More Info' });
		await expect(link).toHaveAttribute(
			'href',
			'https://argocd-test.example.com/applications/argocd/frontend'
		);
		await expect(link).toHaveAttribute('target', '_blank');
	});

	test('the Manifests accordion is collapsed by default and expanding reveals the fixture YAML', async ({
		page
	}) => {
		// "name: example" is unique to the full targetManifests YAML (unlike
		// "color: green", which the always-visible diff codeblock above the
		// accordion also renders as a "+ color: green" addition line).
		// svhighlight's CodeBlock renders the text twice (an invisible
		// unhighlighted <code> alongside the visible highlighted one), so
		// scope to the visible occurrence specifically.
		const manifestsHeader = page.getByRole('button', { name: 'Manifests' });
		const manifestText = page.getByText('name: example').filter({ visible: true });
		await expect(manifestsHeader).toBeVisible();
		await expect(manifestText).not.toBeVisible();

		await manifestsHeader.click();
		await expect(manifestText).toBeVisible();
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

		await expect(page.getByRole('heading', { name: 'Status', level: 3 })).toBeVisible();
		const countAfterLoad = diffRequestCount;
		expect(countAfterLoad).toBeGreaterThan(0);

		await page.getByRole('button', { name: 'Reload diff' }).click();

		await expect.poll(() => diffRequestCount).toBeGreaterThan(countAfterLoad);
	});
});

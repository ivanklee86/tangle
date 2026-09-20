import { expect, test } from '@playwright/test';

test.describe('diffs page (live)', () => {
	test('nested tabs render for every application, manifests expand with real YAML, reload re-issues a real request, and no page errors occur', async ({
		page
	}) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/diffs/?targetRef=test_gitops');

		// Only the active outer tab's inner tablist is mounted at a time, so
		// this stays exactly two tablists — outer (ArgoCD) and inner
		// (application) — no matter which outer tab is currently selected.
		const outerTabs = page.getByRole('tablist').first().getByRole('tab');
		await expect(outerTabs.first()).toBeVisible({ timeout: 30_000 });
		const outerTabCount = await outerTabs.count();
		expect(outerTabCount).toBeGreaterThan(0);

		for (let i = 0; i < outerTabCount; i++) {
			await outerTabs.nth(i).click();

			const innerTabs = page.getByRole('tablist').nth(1).getByRole('tab');
			await expect(innerTabs.first()).toBeVisible();
			expect(await innerTabs.count()).toBeGreaterThan(0);
		}

		// Settle back on the first ArgoCD/application before checking content.
		await outerTabs.first().click();
		await expect(page.getByRole('heading', { name: 'Status', level: 3 })).toBeVisible();

		// Manifests accordion: collapsed by default (aria-expanded=false),
		// expands to real YAML on click. svhighlight renders one
		// <code class="language-yaml"> per line, so this checks a non-zero
		// count rather than a single element's text (matching text like
		// "apiVersion:" isn't unique here — the always-visible diff codeblock
		// above the accordion can render the same context lines).
		const manifestsHeader = page.getByRole('button', { name: 'Manifests' });
		await expect(manifestsHeader).toBeVisible();
		await expect(manifestsHeader).toHaveAttribute('aria-expanded', 'false');

		const manifestYamlLines = page.locator('code.language-yaml:visible');
		await expect(manifestYamlLines).toHaveCount(0);

		await manifestsHeader.click();
		await expect(manifestsHeader).toHaveAttribute('aria-expanded', 'true');
		await expect(async () => {
			expect(await manifestYamlLines.count()).toBeGreaterThan(0);
		}).toPass();

		// Reload diff re-issues a real request against the live server.
		const reloadResponse = page.waitForResponse(
			(response) => response.url().includes('/diffs') && response.request().method() === 'POST'
		);
		await page.getByRole('button', { name: 'Reload diff' }).click();
		const response = await reloadResponse;
		expect(response.ok()).toBe(true);

		expect(errors).toEqual([]);
	});
});

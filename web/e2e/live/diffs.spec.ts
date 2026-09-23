import { expect, test } from '@playwright/test';

test.describe('diffs page (live)', () => {
	test('lists every application, shows a real diff, reload re-issues a request, and no page errors occur', async ({
		page
	}) => {
		const errors: Error[] = [];
		page.on('pageerror', (error) => errors.push(error));

		await page.goto('/diffs/?labels=foo:bar&targetRef=test_gitops');

		// The sidebar fills from the applications call, before any diff has
		// come back — that responsiveness is the point of listing rows as
		// pending rather than waiting for the whole fan-out.
		const sidebar = page.getByRole('navigation', { name: 'Applications' });
		await expect(sidebar.getByRole('button').first()).toBeVisible({ timeout: 30_000 });
		const applicationCount = await sidebar.getByRole('button').count();
		expect(applicationCount).toBeGreaterThan(0);

		// The detail pane opens on the first application without a click.
		await expect(page.getByLabel('Diff location')).toBeVisible();
		await expect(page.getByRole('tab', { name: /^Diff/ })).toBeVisible({ timeout: 30_000 });

		// Real YAML behind the manifest tabs. svhighlight renders one
		// <code class="language-yaml"> per line, so this checks a non-zero
		// count rather than one element's text.
		await page.getByRole('tab', { name: 'Target manifests' }).click();
		const yamlLines = page.locator('code.language-yaml:visible');
		await expect(async () => {
			expect(await yamlLines.count()).toBeGreaterThan(0);
		}).toPass();

		// Reload diff re-issues a real request against the live server.
		const reloadResponse = page.waitForResponse(
			(response) => response.url().includes('/diffs') && response.request().method() === 'POST'
		);
		await page.getByRole('button', { name: 'Reload diff' }).click();
		expect((await reloadResponse).ok()).toBe(true);

		expect(errors).toEqual([]);
	});

	test('selecting another application swaps the detail pane', async ({ page }) => {
		await page.goto('/diffs/?labels=foo:bar&targetRef=test_gitops');

		const sidebar = page.getByRole('navigation', { name: 'Applications' });
		const buttons = sidebar.getByRole('button');
		await expect(buttons.first()).toBeVisible({ timeout: 30_000 });

		if ((await buttons.count()) < 2) test.skip();

		const secondName = (await buttons.nth(1).innerText()).split('\n')[0].trim();
		await buttons.nth(1).click();

		await expect(page.getByRole('heading', { name: secondName, level: 2 })).toBeVisible();
	});

	// ADR 0008: this page fans out one diff-generation POST per application to
	// every ArgoCD, so a bare visit must not start it.
	test('a visit without a target ref opens the query editor instead', async ({ page }) => {
		await page.goto('/diffs/?labels=foo:bar');

		await expect(page.getByRole('heading', { name: 'Edit diff query', level: 2 })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Apply and run diffs' })).toBeDisabled();
	});
});

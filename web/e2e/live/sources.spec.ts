import { expect, test, type Page } from '@playwright/test';

// The source-type fixture Applications (integration/kubernetes/sources/),
// seen through the UI. They're only visible to the `sources` ArgoCD in
// integration/tangle.yaml, so they never show up in the scenario specs.
// What each one renders at main and test_gitops is pinned against ArgoCD by
// TestE2E_SourceFixtures (internal/argocd/fixtures_e2e_test.go):
//
//   - tanka: a config management plugin app; the change modifies one
//     ConfigMap line and removes another.
//   - helm-repo: a chart from a Helm repo; its refs are chart versions, so a
//     git branch fails to render.
//   - helm-repo-git-values, git-chart-git-values, two-repos: multi-source
//     apps, each with a real change on test_gitops.

const SOURCES = ['git-chart-git-values', 'helm-repo', 'helm-repo-git-values', 'tanka', 'two-repos'];
const MULTI_SOURCE = ['git-chart-git-values', 'helm-repo-git-values', 'two-repos'];

// A real ArgoCD manifest render per application.
const DIFFS_TIMEOUT = 60_000;

function collectPageErrors(page: Page): Error[] {
	const errors: Error[] = [];
	page.on('pageerror', (error) => errors.push(error));
	return errors;
}

function applicationRow(page: Page, name: string) {
	return page.locator('tbody tr').filter({ has: page.getByRole('cell', { name, exact: true }) });
}

function diffsSidebar(page: Page) {
	return page.getByRole('navigation', { name: 'Applications' });
}

function diffsEntry(page: Page, name: string) {
	return diffsSidebar(page).getByRole('button', { name: new RegExp(`^${name}(\\s|$)`) });
}

async function openDiffs(page: Page, labels: string, targetRef: string, count: number) {
	await page.goto(`/diffs?${new URLSearchParams({ labels, targetRef })}`);
	await expect(page.getByText(`${count} of ${count} loaded`)).toBeVisible({
		timeout: DIFFS_TIMEOUT
	});
}

async function selectApplication(page: Page, name: string) {
	await diffsEntry(page, name).click();
	await expect(page.getByRole('heading', { name, level: 2 })).toBeVisible();
}

function diffLine(page: Page, text: RegExp) {
	return page.locator('code:visible').filter({ hasText: text }).first();
}

test.describe('source-type fixtures (live)', () => {
	test('the Applications page lists every source type on the sources ArgoCD', async ({ page }) => {
		const errors = collectPageErrors(page);

		await page.goto(`/applications?${new URLSearchParams({ labels: 'suite:sources' })}`);

		await expect(page.locator('tbody tr')).toHaveCount(SOURCES.length, { timeout: 30_000 });
		for (const name of SOURCES) {
			const row = applicationRow(page, name);
			await expect(row).toContainText('sources');
			// Every fixture auto-syncs, so each one settles Healthy.
			await expect(async () => {
				await page.reload();
				await expect(row).toContainText('Healthy', { timeout: 10_000 });
				await expect(row).toContainText('Synced');
			}).toPass({ timeout: DIFFS_TIMEOUT });
		}

		// A single-source app's live ref is its targetRevision: a branch, or a
		// chart version for a chart from a Helm repo.
		await expect(applicationRow(page, 'tanka')).toContainText('main');
		await expect(applicationRow(page, 'helm-repo')).toContainText('6.17.0');

		expect(errors).toEqual([]);
	});

	test('the Diffs page shows the Tanka change and the Helm-repo failure', async ({ page }) => {
		const errors = collectPageErrors(page);

		await openDiffs(page, 'suite:sources', 'test_gitops', SOURCES.length);

		// Only the multi-source apps' changes are missing from this count; see
		// the fixme below.
		await expect(page.getByText('1 changed', { exact: true })).toBeVisible();
		await expect(page.getByText('1 error', { exact: true })).toBeVisible();

		// tanka: one line modified, one removed.
		await expect(diffsEntry(page, 'tanka')).toContainText('+1');
		await expect(diffsEntry(page, 'tanka')).toContainText('−2');
		await selectApplication(page, 'tanka');
		await expect(diffLine(page, /greeting: hello, world/)).toBeVisible();
		await expect(diffLine(page, /removed: soon/)).toBeVisible();

		// helm-repo: a branch name isn't a chart version.
		await expect(diffsEntry(page, 'helm-repo')).toContainText('Error');
		await selectApplication(page, 'helm-repo');
		await expect(page.getByText("Couldn't generate manifests")).toBeVisible();
		await expect(page.getByText(/invalid revision.*test_gitops/)).toBeVisible();

		expect(errors).toEqual([]);
	});

	test('a chart version as the target ref diffs a chart from a Helm repo', async ({ page }) => {
		const errors = collectPageErrors(page);

		await openDiffs(page, 'source:helm-repo', '6.16.0', 1);

		await expect(page.getByText('1 changed', { exact: true })).toBeVisible();
		await selectApplication(page, 'helm-repo');
		await expect(diffLine(page, /helm\.sh\/chart: podinfo-6\.16\.0/)).toBeVisible();
		await expect(diffLine(page, /image: ghcr\.io\/stefanprodan\/podinfo:6\.16\.0/)).toBeVisible();

		expect(errors).toEqual([]);
	});

	// Known gap: Tangle sends the pushed ref as Revision, which ArgoCD ignores
	// for a multi-source app, so today each of these shows "No changes". This
	// asserts the behavior users need; remove the fixme once Tangle sends
	// Revisions and SourcePositions (docs/agents/plans/integration-manifests-repo.md).
	test.fixme('the Diffs page shows each multi-source app’s change', async ({ page }) => {
		await openDiffs(page, 'source:multi-source', 'test_gitops', MULTI_SOURCE.length);

		await expect(page.getByText(`${MULTI_SOURCE.length} changed`, { exact: true })).toBeVisible();

		await selectApplication(page, 'helm-repo-git-values');
		await expect(diffLine(page, /replicas: 2/)).toBeVisible();

		await selectApplication(page, 'git-chart-git-values');
		await expect(diffLine(page, /greeting: hello, world/)).toBeVisible();
		await expect(diffLine(page, /extra: b/)).toBeVisible();

		await selectApplication(page, 'two-repos');
		await expect(diffLine(page, /greeting: hello, world/)).toBeVisible();
	});
});

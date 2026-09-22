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

	// The application names once rendered browser-default black on gray-800,
	// because app.html set a background for both modes but no text colour and
	// these spans inherited it. Asserted as a contrast ratio rather than a class
	// name: the palette work (ADR 0026's sibling, the indigo move) was all about
	// text staying readable, and a class can be present and still fail.
	test('sidebar application names are readable against their background', async ({ page }) => {
		const name = page.getByRole('button', { name: /frontend/ });
		await expect(name).toBeVisible();

		const ratio = await name.evaluate((el) => {
			// Resolve colours through a canvas rather than parsing the string:
			// Tailwind v4 emits oklch(), and a regex that assumes rgb() reads
			// those three numbers as RGB bytes and reports nonsense.
			const toRgb = (value: string): number[] => {
				const canvas = document.createElement('canvas');
				canvas.width = 1;
				canvas.height = 1;
				const ctx = canvas.getContext('2d')!;
				ctx.fillStyle = '#000';
				ctx.fillRect(0, 0, 1, 1);
				ctx.fillStyle = value;
				ctx.fillRect(0, 0, 1, 1);
				return [...ctx.getImageData(0, 0, 1, 1).data].slice(0, 3);
			};

			const channel = (v: number) => {
				const c = v / 255;
				return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
			};
			const luminance = (rgb: number[]) =>
				0.2126 * channel(rgb[0]) + 0.7152 * channel(rgb[1]) + 0.0722 * channel(rgb[2]);

			// Walk up for the nearest painted background — a row is transparent
			// until it is hovered or selected.
			let node: HTMLElement | null = el as HTMLElement;
			let background = 'rgba(0, 0, 0, 0)';
			while (node) {
				const value = getComputedStyle(node).backgroundColor;
				if (value !== 'rgba(0, 0, 0, 0)' && value !== 'transparent') {
					background = value;
					break;
				}
				node = node.parentElement;
			}

			const textLuminance = luminance(toRgb(getComputedStyle(el).color));
			const backgroundLuminance = luminance(toRgb(background));
			const lighter = Math.max(textLuminance, backgroundLuminance);
			const darker = Math.min(textLuminance, backgroundLuminance);
			return (lighter + 0.05) / (darker + 0.05);
		});

		// WCAG AA for body text.
		expect(ratio).toBeGreaterThanOrEqual(4.5);
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

	test('the drawer previews both the link and the tangle-cli command', async ({ page }) => {
		await page.getByRole('button', { name: 'Edit query' }).click();

		await expect(page.getByText('/diffs?targetRef=main&labels=foo%3Abar')).toBeVisible();
		await expect(
			page.getByText('tangle-cli generate-manifests --label foo=bar --target-ref main')
		).toBeVisible();
	});

	// Without --target-ref, generate-manifests compares every application's
	// live ref against itself — a request per application for a diff that is
	// empty by construction.
	test('says a target ref is needed when the CLI command has none', async ({ page }) => {
		await page.goto('/diffs/?labels=foo:bar');

		await expect(
			page.getByText('Add a target ref — without one there is nothing to compare against.')
		).toBeVisible();
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
			// Start from a blank page first: beforeEach already navigated with a
			// query, and counting from here would otherwise race that page load's
			// own request into the tally.
			await page.goto('about:blank');
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

import { expect, type Page } from '@playwright/test';
import { mockFleetAPI } from './fleet';

type Theme = 'dark' | 'light';

/**
 * Opens a page on the fixture fleet in the given theme. The theme goes in
 * through the same localStorage key Flowbite's DarkMode reads when it mounts,
 * so the page renders in it the way a user who picked it would see.
 */
async function openScreen(page: Page, path: string, theme: Theme = 'dark'): Promise<void> {
	await page.addInitScript((value) => localStorage.setItem('THEME_PREFERENCE_KEY', value), theme);
	await mockFleetAPI(page);
	await page.goto(path);
}

/** Waits until every diff has come back, so nothing on screen is still moving. */
async function diffsSettled(page: Page): Promise<void> {
	await expect(page.getByText(/(\d+) of \1 loaded/)).toBeVisible({ timeout: 30_000 });
}

/** Selects an application in the Diffs sidebar and waits for its detail pane. */
async function selectDiff(page: Page, name: string): Promise<void> {
	await page
		.getByRole('navigation', { name: 'Applications' })
		.getByRole('button', { name: new RegExp(`^${name}(\\s|$)`) })
		.first()
		.click();
	await expect(page.getByRole('heading', { name, level: 2 })).toBeVisible();
}

/**
 * Moves the pointer off the page before a screenshot, so the last thing
 * clicked doesn't carry a hover highlight into the picture.
 */
async function ready(page: Page): Promise<void> {
	await page.mouse.move(0, 0);
}

/** Diffs page path for a query. */
function diffsPath(query: Record<string, string | string[]>): string {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(query)) {
		for (const item of Array.isArray(value) ? value : [value]) params.append(key, item);
	}
	return `/diffs?${params}`;
}

const THEMES: Theme[] = ['dark', 'light'];
const PLATFORM_VALUES = 'https://github.com/acme/platform-values.git';
const CHECKOUT_API = 'https://github.com/acme/checkout-api.git';
const LEGACY_CONFIG = 'https://github.com/acme/legacy-config.git';

export {
	CHECKOUT_API,
	LEGACY_CONFIG,
	diffsPath,
	diffsSettled,
	openScreen,
	PLATFORM_VALUES,
	ready,
	selectDiff,
	THEMES,
	type Theme
};

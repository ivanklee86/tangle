import { afterEach, describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Header from './Header.svelte';

describe('Header', () => {
	afterEach(() => {
		document.documentElement.classList.remove('dark');
	});

	test('shows the Tangle brand', async () => {
		const screen = await render(Header);

		await expect.element(screen.getByText('Tangle')).toBeVisible();
	});

	test('toggles dark mode on click', async () => {
		document.documentElement.classList.remove('dark');
		const screen = await render(Header);

		const toggle = screen.getByRole('button', { name: 'Dark mode' });

		await toggle.click();
		expect(document.documentElement.classList.contains('dark')).toBe(true);

		await toggle.click();
		expect(document.documentElement.classList.contains('dark')).toBe(false);
	});
});

import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import TargetReposInput from './TargetReposInput.svelte';

// Test cases:
//   - adding shows the repo shortened, with the full URL kept in the value
//   - Enter adds rather than submitting the surrounding form
//   - another spelling of a repo already added is refused, and says why
//   - removing drops just that repo
describe('TargetReposInput', () => {
	const input = (screen: Awaited<ReturnType<typeof render>>) =>
		screen.getByRole('textbox', { name: 'Target repos' });

	test('adds a repo, shown short', async () => {
		const screen = await render(TargetReposInput, { value: [] });

		await input(screen).fill('https://github.com/org/values.git');
		await screen.getByRole('button', { name: 'Add target repo' }).click();

		await expect.element(screen.getByText('org/values')).toBeVisible();
		await expect
			.element(screen.getByRole('button', { name: 'Remove target repo org/values' }))
			.toBeVisible();
	});

	test('Enter adds instead of submitting', async () => {
		const submit = vi.fn((event: SubmitEvent) => event.preventDefault());
		document.addEventListener('submit', submit);
		const screen = await render(TargetReposInput, { value: [] });

		await input(screen).fill('https://github.com/org/values.git');
		input(screen)
			.element()
			.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));

		await expect.element(screen.getByText('org/values')).toBeVisible();
		expect(submit).not.toHaveBeenCalled();
		document.removeEventListener('submit', submit);
	});

	test('refuses another spelling of a repo already added', async () => {
		const screen = await render(TargetReposInput, { value: ['https://github.com/org/values.git'] });

		await input(screen).fill('git@github.com:org/values');

		await expect.element(screen.getByRole('button', { name: 'Add target repo' })).toBeDisabled();
		await expect.element(screen.getByText('org/values is already a target repo.')).toBeVisible();
	});

	test('removes just that repo', async () => {
		const screen = await render(TargetReposInput, {
			value: ['https://github.com/org/values.git', 'https://github.com/org/charts.git']
		});

		await screen.getByRole('button', { name: 'Remove target repo org/values' }).click();

		await expect.element(screen.getByText('org/values')).not.toBeInTheDocument();
		await expect.element(screen.getByText('org/charts')).toBeVisible();
	});

	// Test cases (suggestions from the page's applications):
	//   - the repos they use are offered, most used first, with their counts,
	//     leaving out any already added
	//   - a target repo none of them use gets a warning; one they use doesn't
	//   - with no applications loaded there's nothing to compare, so no warning
	const suggestions = [
		{ repo: 'github.com/org/values', url: 'https://github.com/org/values.git', apps: 9 },
		{ repo: 'github.com/org/charts', url: 'https://github.com/org/charts.git', apps: 1 }
	];

	test('offers the repos the applications use', async () => {
		const screen = await render(TargetReposInput, {
			value: ['git@github.com:org/charts.git'],
			suggestions
		});

		// With suggestions attached, the field is a combobox.
		const field = screen.getByRole('combobox', { name: 'Target repos' });
		const options = [...screen.container.querySelectorAll('datalist option')];
		expect(
			options.map((option) => [option.getAttribute('value'), option.getAttribute('label')])
		).toEqual([['https://github.com/org/values.git', 'org/values · 9 apps']]);
		expect(field.element().getAttribute('list')).toBe(
			screen.container.querySelector('datalist')?.id
		);
	});

	test('warns about a target repo no application uses', async () => {
		const screen = await render(TargetReposInput, {
			value: ['https://github.com/org/values', 'https://github.com/org/typo.git'],
			suggestions
		});

		await expect
			.element(screen.getByText('No application in this query uses org/typo.'))
			.toBeVisible();
	});

	test('says nothing without applications to compare against', async () => {
		const screen = await render(TargetReposInput, { value: ['https://github.com/org/typo.git'] });

		await expect
			.element(screen.getByText(/No application in this query uses/))
			.not.toBeInTheDocument();
		expect(screen.container.querySelector('datalist')).toBeNull();
	});
});

import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import DiffsForm from './DiffsForm.svelte';

describe('DiffsForm', () => {
	test('submits the entered labels and target ref when the button is clicked', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add Labels' }).click();

		await screen.getByPlaceholder('Git branch').fill('main');
		await screen.getByRole('button', { name: 'See diffs' }).click();

		expect(onSubmit).toHaveBeenCalledWith('env:prod', '', 'main');
	});

	test('submits when Enter is pressed in a field, without a button click', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByPlaceholder('Git branch').fill('main');
		await userEvent.keyboard('{Enter}');

		expect(onSubmit).toHaveBeenCalledWith('', '', 'main');
	});

	test('keeps the button disabled while the target ref is only whitespace', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByPlaceholder('Git branch').fill('   ');

		await expect.element(screen.getByRole('button', { name: 'See diffs' })).toBeDisabled();
		expect(onSubmit).not.toHaveBeenCalled();
	});

	test('trims surrounding whitespace from the target ref before submitting', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByPlaceholder('Git branch').fill('  main  ');
		await screen.getByRole('button', { name: 'See diffs' }).click();

		expect(onSubmit).toHaveBeenCalledWith('', '', 'main');
	});

	test('shows a tooltip on the disabled submit button explaining the missing target ref', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByRole('button', { name: 'See diffs' }).hover();

		await expect
			.element(screen.getByText('Enter a target ref (git branch) to see diffs.'))
			.toBeVisible();
	});

	test('shows no target ref tooltip once a target ref is filled in', async () => {
		const onSubmit = vi.fn();
		const screen = await render(DiffsForm, { onSubmit });

		await screen.getByPlaceholder('Git branch').fill('main');
		await screen.getByRole('button', { name: 'See diffs' }).hover();

		await expect
			.element(screen.getByText('Enter a target ref (git branch) to see diffs.'))
			.not.toBeInTheDocument();
	});
});

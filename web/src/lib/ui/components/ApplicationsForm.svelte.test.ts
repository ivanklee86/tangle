import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import ApplicationsForm from './ApplicationsForm.svelte';

describe('ApplicationsForm', () => {
	test('submits the entered labels when the button is clicked', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		await screen.getByPlaceholder("Labels in format 'key:value'").fill('env:prod');
		await screen.getByPlaceholder("Labels to exclude in format 'key:value'").fill('tier:test');
		await screen.getByRole('button', { name: 'See applications' }).click();

		expect(onSubmit).toHaveBeenCalledWith('env:prod', 'tier:test');
	});

	test('submits when Enter is pressed in a field, without a button click', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		const labelsInput = screen.getByPlaceholder("Labels in format 'key:value'");
		await labelsInput.fill('env:prod');
		await userEvent.keyboard('{Enter}');

		expect(onSubmit).toHaveBeenCalledWith('env:prod', '');
	});

	test('disables the button and does not submit when labels are malformed', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		await screen.getByPlaceholder("Labels in format 'key:value'").fill('not-a-label');

		await expect.element(screen.getByRole('button', { name: 'See applications' })).toBeDisabled();
		expect(onSubmit).not.toHaveBeenCalled();
	});
});

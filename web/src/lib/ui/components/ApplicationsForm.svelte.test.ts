import { describe, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import ApplicationsForm from './ApplicationsForm.svelte';

describe('ApplicationsForm', () => {
	test('submits the labels and exclude labels added via LabelsInput when the button is clicked', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add label' }).click();

		await screen.getByRole('textbox', { name: 'Exclude Labels key' }).fill('tier');
		await screen.getByRole('textbox', { name: 'Exclude Labels value' }).fill('test');
		await screen.getByRole('button', { name: 'Add exclusion' }).click();

		await screen.getByRole('button', { name: 'See applications' }).click();

		expect(onSubmit).toHaveBeenCalledWith('env:prod', 'tier:test');
	});

	test('pressing Enter in a LabelsInput box adds a pair instead of submitting the form', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await userEvent.keyboard('{Enter}');

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		expect(onSubmit).not.toHaveBeenCalled();
	});

	test('submits with empty labels by default, since no pairs are required', async () => {
		const onSubmit = vi.fn();
		const screen = await render(ApplicationsForm, { onSubmit });

		await screen.getByRole('button', { name: 'See applications' }).click();

		expect(onSubmit).toHaveBeenCalledWith('', '');
	});
});

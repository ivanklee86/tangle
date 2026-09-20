import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import LabelsInput from './LabelsInput.svelte';

describe('LabelsInput', () => {
	test('adds a pair as a chip when the add button is clicked, and clears the draft boxes', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add Labels' }).click();

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		await expect.element(screen.getByRole('textbox', { name: 'Labels key' })).toHaveValue('');
		await expect.element(screen.getByRole('textbox', { name: 'Labels value' })).toHaveValue('');
	});

	test('adds a pair when Enter is pressed in either draft box, without a button click', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await userEvent.keyboard('{Enter}');

		await expect.element(screen.getByText('env:prod')).toBeVisible();
	});

	test('disables the add button while the key or value box is empty', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await expect.element(screen.getByRole('button', { name: 'Add Labels' })).toBeDisabled();

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await expect.element(screen.getByRole('button', { name: 'Add Labels' })).toBeDisabled();

		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await expect.element(screen.getByRole('button', { name: 'Add Labels' })).not.toBeDisabled();
	});

	test('disables the add button while the key or value contains a reserved character', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env:staging');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');

		await expect.element(screen.getByRole('button', { name: 'Add Labels' })).toBeDisabled();
	});

	test('shows "Value is required" and does not add a pair when Enter is pressed with only a key filled in', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await userEvent.keyboard('{Enter}');

		await expect.element(screen.getByText('Value is required')).toBeVisible();
		await expect.element(screen.getByText('Key is required')).not.toBeInTheDocument();
		await expect.element(screen.getByRole('textbox', { name: 'Labels key' })).toHaveValue('env');
	});

	test('shows "Key is required" when Enter is pressed with only a value filled in', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await userEvent.keyboard('{Enter}');

		await expect.element(screen.getByText('Key is required')).toBeVisible();
		await expect.element(screen.getByText('Value is required')).not.toBeInTheDocument();
	});

	test('clears the "Value is required" message once a value is typed in', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		const keyBox = screen.getByRole('textbox', { name: 'Labels key' });
		await keyBox.fill('env');
		await userEvent.keyboard('{Enter}');
		await expect.element(screen.getByText('Value is required')).toBeVisible();

		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');

		await expect.element(screen.getByText('Value is required')).not.toBeInTheDocument();
	});

	test('does not show a required message before any add attempt', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await expect.element(screen.getByText('Key is required')).not.toBeInTheDocument();
		await expect.element(screen.getByText('Value is required')).not.toBeInTheDocument();
	});

	test('shows a tooltip nudging to fill in both boxes when hovering the add button before anything is typed', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('button', { name: 'Add Labels' }).hover();

		await expect
			.element(screen.getByText('Enter a key and value to add this label.'))
			.toBeVisible();
	});

	test('shows a tooltip nudging to enter the missing value when hovering the add button with only a key filled in', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('button', { name: 'Add Labels' }).hover();

		await expect.element(screen.getByText('Enter a value to add this label.')).toBeVisible();
	});

	test('shows a tooltip about reserved characters when hovering the add button with an invalid pair', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env:staging');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add Labels' }).hover();

		await expect.element(screen.getByText("Key and value can't contain ':' or ','.")).toBeVisible();
	});

	test('shows no tooltip when hovering the add button with a valid pair ready to add', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add Labels' }).hover();

		await expect
			.element(screen.getByText('Enter a key and value to add this label.'))
			.not.toBeInTheDocument();
	});

	test('removes a pair when its chip is dismissed', async () => {
		const screen = await render(LabelsInput, { label: 'Labels' });

		await screen.getByRole('textbox', { name: 'Labels key' }).fill('env');
		await screen.getByRole('textbox', { name: 'Labels value' }).fill('prod');
		await screen.getByRole('button', { name: 'Add Labels' }).click();
		await expect.element(screen.getByText('env:prod')).toBeVisible();

		await screen.getByRole('button', { name: 'Remove badge' }).click();

		await expect.element(screen.getByText('env:prod')).not.toBeInTheDocument();
	});

	test('renders chips for pairs already present in an initial value', async () => {
		const screen = await render(LabelsInput, { label: 'Labels', value: 'env:prod,tier:frontend' });

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		await expect.element(screen.getByText('tier:frontend')).toBeVisible();
	});
});

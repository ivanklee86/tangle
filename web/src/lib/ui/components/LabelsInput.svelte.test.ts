import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import LabelsInput from './LabelsInput.svelte';

const LABEL = 'Include applications with all of these labels';

function boxes(screen: ReturnType<typeof render> extends Promise<infer T> ? T : never) {
	return {
		key: screen.getByRole('textbox', { name: `${LABEL} key` }),
		value: screen.getByRole('textbox', { name: `${LABEL} value` }),
		add: screen.getByRole('button', { name: 'Add label' })
	};
}

describe('LabelsInput', () => {
	test('adds a pair as a chip when the add button is clicked, and clears the draft boxes', async () => {
		const screen = await render(LabelsInput, { label: LABEL });
		const { key, value, add } = boxes(screen);

		await key.fill('env');
		await value.fill('prod');
		await add.click();

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		await expect.element(key).toHaveValue('');
		await expect.element(value).toHaveValue('');
	});

	test('adds a pair when Enter is pressed, without submitting anything', async () => {
		const screen = await render(LabelsInput, { label: LABEL });
		const { key, value } = boxes(screen);

		await key.fill('env');
		await value.fill('prod');
		await userEvent.keyboard('{Enter}');

		await expect.element(screen.getByText('env:prod')).toBeVisible();
	});

	test('disables the add button while the key or value box is empty', async () => {
		const screen = await render(LabelsInput, { label: LABEL });
		const { key, value, add } = boxes(screen);

		await expect.element(add).toBeDisabled();

		await key.fill('env');
		await expect.element(add).toBeDisabled();

		await value.fill('prod');
		await expect.element(add).not.toBeDisabled();
	});

	test('disables the add button while the key or value contains a reserved character', async () => {
		const screen = await render(LabelsInput, { label: LABEL });
		const { key, value, add } = boxes(screen);

		await key.fill('env:staging');
		await value.fill('prod');

		await expect.element(add).toBeDisabled();
	});

	test('explains why a colon or comma is refused, rather than just refusing it', async () => {
		const screen = await render(LabelsInput, { label: LABEL });
		const { key, value } = boxes(screen);

		await key.fill('env:staging');
		await value.fill('prod');
		await userEvent.keyboard('{Enter}');

		await expect
			.element(screen.getByText('Colons and commas separate pairs', { exact: false }))
			.toBeVisible();
	});

	// Each key may appear at most once: the server builds one Kubernetes
	// selector from these and rejects a repeated key with a 400 (ADR 0026), so
	// a UI that let you add one would be offering a query the API refuses.
	describe('per-key uniqueness', () => {
		test('refuses a key that is already a chip, naming it', async () => {
			const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod' });
			const { key, value, add } = boxes(screen);

			await key.fill('env');
			await value.fill('staging');

			await expect.element(add).toBeDisabled();
			await expect
				.element(screen.getByText('"env" is already used', { exact: false }))
				.toBeVisible();
		});

		test('allows the same value under a different key', async () => {
			const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod' });
			const { key, value, add } = boxes(screen);

			await key.fill('tier');
			await value.fill('prod');

			await expect.element(add).not.toBeDisabled();
		});

		test('frees a key again once its chip is removed', async () => {
			const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod' });
			const { key, value, add } = boxes(screen);

			await key.fill('env');
			await value.fill('staging');
			await expect.element(add).toBeDisabled();

			await screen.getByRole('button', { name: 'Remove label env:prod' }).click();

			await expect.element(add).not.toBeDisabled();
		});
	});

	describe('feedback on a failed add', () => {
		test('says a value is needed when Enter is pressed with only a key', async () => {
			const screen = await render(LabelsInput, { label: LABEL });
			const { key } = boxes(screen);

			await key.fill('env');
			await userEvent.keyboard('{Enter}');

			await expect.element(screen.getByText('Enter a value to add this label.')).toBeVisible();
			await expect.element(key).toHaveValue('env');
		});

		test('says a key is needed when Enter is pressed with only a value', async () => {
			const screen = await render(LabelsInput, { label: LABEL });
			const { value } = boxes(screen);

			await value.fill('prod');
			await userEvent.keyboard('{Enter}');

			await expect.element(screen.getByText('Enter a key to add this label.')).toBeVisible();
		});

		test('clears the message once the missing box is filled in', async () => {
			const screen = await render(LabelsInput, { label: LABEL });
			const { key, value } = boxes(screen);

			await key.fill('env');
			await userEvent.keyboard('{Enter}');
			await expect.element(screen.getByText('Enter a value to add this label.')).toBeVisible();

			await value.fill('prod');

			await expect
				.element(screen.getByText('Enter a value to add this label.'))
				.not.toBeInTheDocument();
		});

		test('shows only the resting instruction before any add attempt', async () => {
			const screen = await render(LabelsInput, { label: LABEL });

			await expect.element(screen.getByText('Press Enter to add.', { exact: false })).toBeVisible();
			await expect
				.element(screen.getByText('Enter a key and value to add this label.'))
				.not.toBeInTheDocument();
		});
	});

	test('removes a pair when its chip is dismissed', async () => {
		const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod' });

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		await screen.getByRole('button', { name: 'Remove label env:prod' }).click();

		await expect.element(screen.getByText('env:prod')).not.toBeInTheDocument();
	});

	test('renders chips for pairs already present in an initial value', async () => {
		const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod,tier:frontend' });

		await expect.element(screen.getByText('env:prod')).toBeVisible();
		await expect.element(screen.getByText('tier:frontend')).toBeVisible();
	});

	test('counts the chips beside the heading', async () => {
		const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod,tier:frontend' });

		await expect.element(screen.getByText('2 labels')).toBeVisible();
	});

	// A link whose query can't be used in full is a wider query than the
	// sender described. Reporting it is the UI half of the silent-drop fix
	// that #241 made on the server.
	describe('an initial value with unusable segments', () => {
		test('reports a malformed segment rather than quietly dropping it', async () => {
			const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod,not-a-pair' });

			await expect.element(screen.getByText('env:prod')).toBeVisible();
			await expect
				.element(screen.getByText(`"not-a-pair" isn't key:value`, { exact: false }))
				.toBeVisible();
		});

		test('reports a repeated key and keeps the first occurrence', async () => {
			const screen = await render(LabelsInput, { label: LABEL, value: 'env:prod,env:staging' });

			await expect.element(screen.getByText('env:prod')).toBeVisible();
			await expect.element(screen.getByText('env:staging')).not.toBeInTheDocument();
			await expect
				.element(screen.getByText('"env" appeared more than once', { exact: false }))
				.toBeVisible();
		});
	});
});

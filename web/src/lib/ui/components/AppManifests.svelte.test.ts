import { describe, expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import AppManifests from './AppManifests.svelte';
import { type ApplicationDiff } from '$lib/backend/data';

function makeDiff(overrides: Partial<ApplicationDiff> = {}): ApplicationDiff {
	return {
		response: { liveManifests: '', targetManifests: '', diffs: '', manifestGenerationError: '' },
		errorResponse: { error: '' },
		requestDetails: { argoCD: 'test', applicationName: 'my-app' },
		error: false,
		loaded: false,
		...overrides
	};
}

describe('AppManifests', () => {
	test('shows the system error alert when the request failed', async () => {
		const diffData = makeDiff({ error: true, errorResponse: { error: 'boom' } });

		const screen = await render(AppManifests, { diffData });

		await expect.element(screen.getByText('System error!')).toBeVisible();
		await expect.element(screen.getByText(/boom/)).toBeVisible();
	});

	test('shows the manifest generation error when present', async () => {
		const diffData = makeDiff({
			loaded: true,
			response: {
				liveManifests: '',
				targetManifests: '',
				diffs: '',
				manifestGenerationError: 'bad manifest'
			}
		});

		const screen = await render(AppManifests, { diffData });

		await expect.element(screen.getByText('Error generating manifests!')).toBeVisible();
		await expect.element(screen.getByText('bad manifest')).toBeVisible();
	});

	test('shows "No diffs found." when loaded with no diffs', async () => {
		const diffData = makeDiff({ loaded: true });

		const screen = await render(AppManifests, { diffData });

		await expect.element(screen.getByText('No diffs found.')).toBeVisible();
	});

	test('renders diffs and a collapsed Manifests accordion when loaded with diffs', async () => {
		const diffData = makeDiff({
			loaded: true,
			response: {
				liveManifests: '',
				targetManifests: 'kind: Pod',
				diffs: '-old\n+new',
				manifestGenerationError: ''
			}
		});

		const screen = await render(AppManifests, { diffData });

		await expect.element(screen.getByText('Diffs')).toBeVisible();
		await expect.element(screen.getByText('Manifests')).toBeVisible();
		await expect.element(screen.getByText(/\+new/).first()).toBeVisible();
	});
});

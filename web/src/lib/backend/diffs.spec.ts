import { describe, it, expect, vi } from 'vitest';
import { buildDiffRequests, buildDiffMap, fetchDiffs } from '$lib/backend/diffs';
import { type ArgoCDApplicationResults, type ApplicationDiff } from '$lib/backend/data';

function makeResults(): ArgoCDApplicationResults[] {
	return [
		{
			name: 'test',
			link: 'https://test.example.com',
			applications: [
				{
					name: 'app-a',
					url: 'https://a',
					health: 'Healthy',
					syncStatus: 'Synced',
					liveRef: 'main'
				},
				{
					name: 'app-b',
					url: 'https://b',
					health: 'Healthy',
					syncStatus: 'Synced',
					liveRef: 'main'
				}
			]
		},
		{
			name: 'prod',
			link: 'https://prod.example.com',
			applications: [
				{
					name: 'app-c',
					url: 'https://c',
					health: 'Healthy',
					syncStatus: 'Synced',
					liveRef: 'main'
				}
			]
		}
	];
}

function makeDiff(argoCD: string, applicationName: string): ApplicationDiff {
	return {
		response: { liveManifests: '', targetManifests: '', diffs: '', manifestGenerationError: '' },
		errorResponse: { error: '' },
		requestDetails: { argoCD, applicationName },
		error: false,
		loaded: true
	};
}

describe('buildDiffRequests', () => {
	it('flattens every ArgoCD/application pair into a request', () => {
		const requests = buildDiffRequests(makeResults(), 'feature-branch');

		expect(requests).toEqual([
			{
				argoCDName: 'test',
				applicationName: 'app-a',
				liveRef: 'main',
				targetRef: 'feature-branch'
			},
			{
				argoCDName: 'test',
				applicationName: 'app-b',
				liveRef: 'main',
				targetRef: 'feature-branch'
			},
			{ argoCDName: 'prod', applicationName: 'app-c', liveRef: 'main', targetRef: 'feature-branch' }
		]);
	});

	it('falls back to liveRef when targetRef is null or empty', () => {
		const [withNull] = buildDiffRequests(makeResults(), null);
		expect(withNull.targetRef).toBe('main');

		const [withEmpty] = buildDiffRequests(makeResults(), '');
		expect(withEmpty.targetRef).toBe('main');
	});
});

describe('buildDiffMap', () => {
	it('groups a flat diff list back into the nested argoCD/application shape', () => {
		const diffs = [makeDiff('test', 'app-a'), makeDiff('test', 'app-b'), makeDiff('prod', 'app-c')];

		const map = buildDiffMap(diffs);

		expect(Object.keys(map)).toEqual(['test', 'prod']);
		expect(map['test']['app-a']).toBe(diffs[0]);
		expect(map['test']['app-b']).toBe(diffs[1]);
		expect(map['prod']['app-c']).toBe(diffs[2]);
	});
});

describe('fetchDiffs', () => {
	it('reports the total before resolving, progress per result, and returns the built map', async () => {
		const getApplicationDiff = vi
			.fn()
			.mockImplementation((argoCD: string, applicationName: string) =>
				Promise.resolve(makeDiff(argoCD, applicationName))
			);
		const client = { getApplicationDiff } as unknown as import('$lib/backend/client').default;

		const onTotal = vi.fn();
		const onProgress = vi.fn();

		const map = await fetchDiffs(client, makeResults(), 'feature-branch', { onTotal, onProgress });

		expect(onTotal).toHaveBeenCalledTimes(1);
		expect(onTotal).toHaveBeenCalledWith(3);
		expect(onProgress).toHaveBeenCalledTimes(3);
		expect(getApplicationDiff).toHaveBeenCalledTimes(3);
		expect(map['test']['app-a'].requestDetails).toEqual({
			argoCD: 'test',
			applicationName: 'app-a'
		});
		expect(map['prod']['app-c'].requestDetails).toEqual({
			argoCD: 'prod',
			applicationName: 'app-c'
		});
	});
});

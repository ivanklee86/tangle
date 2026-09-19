import { describe, it, expect } from 'vitest';
import { filterOutZeroResults } from '$lib/ui/utils';
import { type ArgoCDApplicationResults } from '$lib/backend/data';

describe('filterOutZeroResults', () => {
	it('drops ArgoCD entries with no applications', () => {
		const results: ArgoCDApplicationResults[] = [
			{ name: 'empty', link: '', applications: [] },
			{
				name: 'has-apps',
				link: '',
				applications: [
					{ name: 'a', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
				]
			}
		];

		expect(filterOutZeroResults(results).map((r) => r.name)).toEqual(['has-apps']);
	});

	it('returns an empty array when every entry has no applications', () => {
		const results: ArgoCDApplicationResults[] = [{ name: 'empty', link: '', applications: [] }];

		expect(filterOutZeroResults(results)).toEqual([]);
	});
});

import { describe, it, expect } from 'vitest';
import { emptyApplicationResponseStore } from '$lib/backend/data';

describe('emptyApplicationResponseStore', () => {
	it('returns the expected default shape', () => {
		expect(emptyApplicationResponseStore()).toEqual({
			response: { results: [] },
			errorResponse: { error: '' },
			error: false,
			loaded: false
		});
	});

	it('returns a fresh object each call', () => {
		const first = emptyApplicationResponseStore();
		first.response.results.push({ name: 'mutated', link: '', applications: [] });

		const second = emptyApplicationResponseStore();

		expect(second.response.results).toEqual([]);
	});
});

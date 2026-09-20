import { describe, it, expect } from 'vitest';
import { parseLabels, serializeLabels } from '$lib/ui/labels';

describe('parseLabels', () => {
	it('returns an empty array for an empty string', () => {
		expect(parseLabels('')).toEqual([]);
	});

	it('parses a single key:value pair', () => {
		expect(parseLabels('env:prod')).toEqual([{ key: 'env', value: 'prod' }]);
	});

	it('parses multiple comma-separated pairs in order', () => {
		expect(parseLabels('env:prod,tier:frontend')).toEqual([
			{ key: 'env', value: 'prod' },
			{ key: 'tier', value: 'frontend' }
		]);
	});

	it('drops a malformed segment and keeps the well-formed ones around it', () => {
		expect(parseLabels('env:prod,not-a-pair,tier:frontend')).toEqual([
			{ key: 'env', value: 'prod' },
			{ key: 'tier', value: 'frontend' }
		]);
	});
});

describe('serializeLabels', () => {
	it('returns an empty string for no pairs', () => {
		expect(serializeLabels([])).toBe('');
	});

	it('joins pairs with commas and colons', () => {
		expect(
			serializeLabels([
				{ key: 'env', value: 'prod' },
				{ key: 'tier', value: 'frontend' }
			])
		).toBe('env:prod,tier:frontend');
	});

	it('round-trips through parseLabels', () => {
		const original = 'env:prod,tier:frontend';
		expect(serializeLabels(parseLabels(original))).toBe(original);
	});
});

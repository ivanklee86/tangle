import { describe, it, expect } from 'vitest';
import { parseLabels, parseLabelsStrict, serializeLabels } from '$lib/ui/labels';

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

describe('parseLabelsStrict', () => {
	// The server rejects these outright with a 400 (ADR 0026). The UI keeps
	// what it can so there is something to edit, but it has to be able to say
	// what it threw away — a link that renders fewer chips than it contained
	// is a wider query than its sender described.
	it('reports a malformed segment instead of silently dropping it', () => {
		const parsed = parseLabelsStrict('env:prod,not-a-pair,tier:frontend');

		expect(parsed.pairs).toEqual([
			{ key: 'env', value: 'prod' },
			{ key: 'tier', value: 'frontend' }
		]);
		expect(parsed.invalid).toEqual(['not-a-pair']);
	});

	it('reports a segment with too many separators', () => {
		expect(parseLabelsStrict('env:test:extra').invalid).toEqual(['env:test:extra']);
	});

	it('reports an empty key or value', () => {
		expect(parseLabelsStrict(':value').invalid).toEqual([':value']);
		expect(parseLabelsStrict('key:').invalid).toEqual(['key:']);
	});

	it('keeps the first occurrence of a repeated key and names the key', () => {
		const parsed = parseLabelsStrict('env:prod,env:staging');

		expect(parsed.pairs).toEqual([{ key: 'env', value: 'prod' }]);
		expect(parsed.duplicateKeys).toEqual(['env']);
	});

	it('names a repeated key once however often it repeats', () => {
		expect(parseLabelsStrict('env:a,env:b,env:c').duplicateKeys).toEqual(['env']);
	});

	it('reports nothing for a clean query', () => {
		const parsed = parseLabelsStrict('env:prod,tier:frontend');

		expect(parsed.invalid).toEqual([]);
		expect(parsed.duplicateKeys).toEqual([]);
	});

	it('reports nothing for an empty string', () => {
		expect(parseLabelsStrict('')).toEqual({ pairs: [], invalid: [], duplicateKeys: [] });
	});
});

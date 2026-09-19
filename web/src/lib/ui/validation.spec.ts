import { describe, it, expect } from 'vitest';
import { isValidLabelFormat } from '$lib/ui/validation';

describe('isValidLabelFormat', () => {
	it('treats an empty string as valid (nothing to check)', () => {
		expect(isValidLabelFormat('')).toBe(true);
	});

	it('accepts a single key:value pair', () => {
		expect(isValidLabelFormat('foo:bar')).toBe(true);
	});

	it('accepts multiple comma-separated key:value pairs', () => {
		expect(isValidLabelFormat('foo:bar,baz:qux')).toBe(true);
	});

	it('rejects a pair missing a colon', () => {
		expect(isValidLabelFormat('foobar')).toBe(false);
	});

	it('rejects an empty segment between commas', () => {
		expect(isValidLabelFormat('foo:bar,,baz:qux')).toBe(false);
	});

	it('rejects a trailing stray comma', () => {
		expect(isValidLabelFormat('foo:bar,')).toBe(false);
	});
});

import { describe, it, expect } from 'vitest';
import { duplicateLabelKeys, isValidLabelFormat, isValidLabelQuery } from '$lib/ui/validation';

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

describe('isValidLabelQuery', () => {
	// Stricter than isValidLabelFormat: the server builds one Kubernetes
	// selector from these and rejects a repeated key with a 400 (ADR 0026), so
	// a query the UI calls valid must be one the API will actually run.
	it('accepts a well-formed query with distinct keys', () => {
		expect(isValidLabelQuery('env:test,team:platform')).toBe(true);
	});

	it('accepts an empty query', () => {
		expect(isValidLabelQuery('')).toBe(true);
	});

	it('rejects a repeated key even though the format is fine', () => {
		expect(isValidLabelFormat('env:test,env:prod')).toBe(true);
		expect(isValidLabelQuery('env:test,env:prod')).toBe(false);
	});

	it('allows the same value under different keys', () => {
		expect(isValidLabelQuery('env:shared,team:shared')).toBe(true);
	});

	it('still rejects a malformed segment', () => {
		expect(isValidLabelQuery('env')).toBe(false);
	});
});

describe('duplicateLabelKeys', () => {
	it('names every repeated key, once each', () => {
		expect(duplicateLabelKeys('env:a,env:b,env:c,team:x,team:y')).toEqual(['env', 'team']);
	});

	it('is empty for a query with distinct keys', () => {
		expect(duplicateLabelKeys('env:test,team:platform')).toEqual([]);
	});
});

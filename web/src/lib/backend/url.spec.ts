import { describe, it, expect } from 'vitest';
import { buildQuery } from '#lib/backend/url.ts';

describe('buildQuery', () => {
	it('returns an empty string when there are no usable params', () => {
		expect(buildQuery({})).toBe('');
		expect(buildQuery({ labels: null, excludeLabels: undefined, targetRef: '' })).toBe('');
	});

	it('builds a single-param query string', () => {
		expect(buildQuery({ labels: 'foo:bar' })).toBe('?labels=foo%3Abar');
	});

	it('joins multiple params in insertion order', () => {
		expect(buildQuery({ targetRef: 'main', labels: 'foo:bar', excludeLabels: 'baz:qux' })).toBe(
			'?targetRef=main&labels=foo%3Abar&excludeLabels=baz%3Aqux'
		);
	});

	it('omits null, undefined, and empty-string values', () => {
		expect(buildQuery({ targetRef: 'main', labels: null, excludeLabels: '' })).toBe(
			'?targetRef=main'
		);
	});

	it('repeats a list parameter once per item, in order, skipping empty items', () => {
		expect(
			buildQuery({ targetRef: 'main', targetRepo: ['https://github.com/org/a', '', 'b'] })
		).toBe('?targetRef=main&targetRepo=https%3A%2F%2Fgithub.com%2Forg%2Fa&targetRepo=b');
	});

	it('omits an empty list', () => {
		expect(buildQuery({ targetRef: 'main', targetRepo: [] })).toBe('?targetRef=main');
	});
});

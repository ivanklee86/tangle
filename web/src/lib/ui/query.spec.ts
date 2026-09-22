import { describe, it, expect } from 'vitest';
import {
	applicationsHref,
	cliCommand,
	diffsHref,
	hasSubmittedQuery,
	isEmptyQuery,
	queryFromParams,
	type Query
} from '$lib/ui/query';

function query(overrides: Partial<Query> = {}): Query {
	return { labels: '', excludeLabels: '', targetRef: '', ...overrides };
}

describe('queryFromParams', () => {
	it('reads a full query back out of a URL', () => {
		const params = new URLSearchParams('labels=env:test&excludeLabels=tier:sandbox&targetRef=main');

		expect(queryFromParams(params)).toEqual({
			labels: 'env:test',
			excludeLabels: 'tier:sandbox',
			targetRef: 'main'
		});
	});

	it('treats a missing parameter as empty rather than undefined', () => {
		// Callers bind these straight into text inputs; undefined would render
		// the string "undefined".
		expect(queryFromParams(new URLSearchParams())).toEqual({
			labels: '',
			excludeLabels: '',
			targetRef: ''
		});
	});
});

describe('isEmptyQuery', () => {
	it('is true when no labels were asked for', () => {
		expect(isEmptyQuery(query())).toBe(true);
	});

	it('ignores the target ref, which does not narrow anything by itself', () => {
		expect(isEmptyQuery(query({ targetRef: 'main' }))).toBe(true);
	});

	it('is false once either label parameter is set', () => {
		expect(isEmptyQuery(query({ labels: 'env:test' }))).toBe(false);
		expect(isEmptyQuery(query({ excludeLabels: 'tier:sandbox' }))).toBe(false);
	});
});

describe('applicationsHref', () => {
	it('carries both label parameters', () => {
		expect(applicationsHref(query({ labels: 'env:test', excludeLabels: 'tier:sandbox' }))).toBe(
			'/applications?labels=env%3Atest&excludeLabels=tier%3Asandbox'
		);
	});

	// "No labels" is a real query meaning "show me everything", but an empty
	// label filter vanishes from the query string — so without a marker a
	// submitted empty query and a bare nav click produce the same URL, and
	// the page can only read it as "nothing asked for yet".
	it('marks a submitted empty query so it can be told from a bare visit', () => {
		expect(applicationsHref(query())).toBe('/applications?searched=true');
	});

	it('leaves the marker off when labels already say a query was submitted', () => {
		expect(applicationsHref(query({ labels: 'env:test' }))).toBe('/applications?labels=env%3Atest');
	});

	it('leaves the target ref out — it means nothing on this page', () => {
		expect(applicationsHref(query({ targetRef: 'main' }))).toBe('/applications?searched=true');
	});
});

describe('hasSubmittedQuery', () => {
	it('is false for a bare visit', () => {
		expect(hasSubmittedQuery(new URLSearchParams())).toBe(false);
	});

	it('is true for either label parameter', () => {
		expect(hasSubmittedQuery(new URLSearchParams('labels=env:test'))).toBe(true);
		expect(hasSubmittedQuery(new URLSearchParams('excludeLabels=tier:sandbox'))).toBe(true);
	});

	it('is true for an explicitly submitted empty query', () => {
		expect(hasSubmittedQuery(new URLSearchParams('searched=true'))).toBe(true);
	});

	it('ignores parameters that narrow nothing', () => {
		expect(hasSubmittedQuery(new URLSearchParams('refresh=true'))).toBe(false);
	});

	it('round-trips with applicationsHref for an empty query', () => {
		const href = applicationsHref(query());
		expect(hasSubmittedQuery(new URLSearchParams(href.split('?')[1]))).toBe(true);
	});
});

describe('diffsHref', () => {
	it('carries the target ref alongside the labels', () => {
		const href = diffsHref(query({ labels: 'env:test', targetRef: 'release-25' }));

		expect(href).toContain('targetRef=release-25');
		expect(href).toContain('labels=env%3Atest');
	});
});

describe('cliCommand', () => {
	// The CLI takes key=value per repeated flag; the API takes key:value in one
	// comma-separated parameter. This is a translation, and getting it wrong
	// hands someone a command that silently filters differently.
	it('translates labels into repeated --label flags', () => {
		expect(cliCommand(query({ labels: 'env:test,team:platform' }))).toBe(
			'tangle-cli generate-manifests --label env=test --label team=platform'
		);
	});

	it('translates exclusions into --exclude-label flags', () => {
		expect(cliCommand(query({ excludeLabels: 'tier:sandbox' }))).toBe(
			'tangle-cli generate-manifests --exclude-label tier=sandbox'
		);
	});

	it('appends the target ref when there is one', () => {
		expect(cliCommand(query({ labels: 'env:test', targetRef: 'release-25' }))).toBe(
			'tangle-cli generate-manifests --label env=test --target-ref release-25'
		);
	});

	it('omits the target ref flag entirely when there is none', () => {
		expect(cliCommand(query({ labels: 'env:test' }))).not.toContain('--target-ref');
	});

	it('is still a runnable command for an empty query', () => {
		expect(cliCommand(query())).toBe('tangle-cli generate-manifests');
	});
});

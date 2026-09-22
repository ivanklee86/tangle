import { describe, it, expect } from 'vitest';
import {
	applicationsHref,
	cliCommand,
	diffsHref,
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

	it('omits empty parameters instead of sending blanks', () => {
		expect(applicationsHref(query())).toBe('/applications');
	});

	it('leaves the target ref out — it means nothing on this page', () => {
		expect(applicationsHref(query({ targetRef: 'main' }))).toBe('/applications');
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

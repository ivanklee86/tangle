import { describe, it, expect, vi } from 'vitest';
import { load } from './+page';
import { type ApplicationResponseStore } from '$lib/backend/data';
import { type Query } from '$lib/ui/query';

function mockFetch(status: number, body: unknown) {
	return vi.fn().mockResolvedValue({
		status,
		text: () => Promise.resolve(JSON.stringify(body))
	});
}

function makeEvent(url: string, fetch: typeof globalThis.fetch) {
	return { url: new URL(url), fetch } as Parameters<typeof load>[0];
}

// `load`'s exported type widens its return to SvelteKit's generic `PageLoad`
// signature (which allows `void`), so calls need this cast back to what the
// concrete implementation actually returns.
function callLoad(url: string, fetch: typeof globalThis.fetch) {
	return load(makeEvent(url, fetch)) as {
		query: Query;
		applications: Promise<ApplicationResponseStore> | undefined;
	};
}

describe('diffs +page.ts load', () => {
	// ADR 0008: this page fans out one diff-generation POST per application to
	// every configured Argo CD, so it must not start on a bare visit. A query
	// with no target ref counts as nothing asked for too — there would be
	// nothing to compare against.
	describe('with nothing to diff', () => {
		it('does not call the API when the URL has no query at all', () => {
			const fetch = mockFetch(200, { results: [] });

			const { applications } = callLoad('http://localhost/diffs/', fetch);

			expect(applications).toBeUndefined();
			expect(fetch).not.toHaveBeenCalled();
		});

		it('does not call the API when labels are set but no target ref is', () => {
			const fetch = mockFetch(200, { results: [] });

			const { applications } = callLoad('http://localhost/diffs/?labels=foo:bar', fetch);

			expect(applications).toBeUndefined();
			expect(fetch).not.toHaveBeenCalled();
		});

		it('still hands the page the query so it can seed its editor', () => {
			const { query } = callLoad('http://localhost/diffs/?labels=foo:bar', mockFetch(200, {}));

			expect(query).toEqual({ labels: 'foo:bar', excludeLabels: '', targetRef: '' });
		});
	});

	// The ref alone is enough: it can only get into the URL by being
	// submitted, so ADR 0008's concern is covered without also demanding
	// labels. "Diff everything against this ref" is a real request, and
	// refusing it would leave the editor reopening forever.
	it('runs with a target ref and no labels', async () => {
		const fetch = mockFetch(200, { results: [] });

		const { applications } = callLoad('http://localhost/diffs/?targetRef=main', fetch);
		await applications;

		expect(fetch).toHaveBeenCalledTimes(1);
		const [url] = fetch.mock.calls[0];
		expect(url).not.toContain('labels');
	});

	it('forwards labels and excludeLabels from the URL, ignoring targetRef', async () => {
		const fetch = mockFetch(200, { results: [] });

		const { applications } = callLoad(
			'http://localhost/diffs/?labels=foo:bar&excludeLabels=baz:qux&targetRef=main',
			fetch
		);
		await applications;

		const [url] = fetch.mock.calls[0];
		expect(url).toContain('labels=foo%3Abar');
		expect(url).toContain('excludeLabels=baz%3Aqux');
		// The diff fan-out uses the ref; the applications lookup has no use
		// for it and the API would reject an unknown parameter's presence as
		// noise in the link.
		expect(url).not.toContain('targetRef');
	});

	it('passes the whole query through for the page header to render', () => {
		const { query } = callLoad(
			'http://localhost/diffs/?labels=foo:bar&targetRef=release-25',
			mockFetch(200, {})
		);

		expect(query).toEqual({ labels: 'foo:bar', excludeLabels: '', targetRef: 'release-25' });
	});

	it('resolves to the parsed applications response', async () => {
		const fetch = mockFetch(200, { results: [{ name: 'test', link: '', applications: [] }] });

		const { applications } = callLoad(
			'http://localhost/diffs/?labels=foo:bar&targetRef=main',
			fetch
		);
		const result = await applications;

		expect(result?.error).toBe(false);
		expect(result?.response.results).toHaveLength(1);
	});
});

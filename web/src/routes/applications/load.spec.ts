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

describe('applications +page.ts load', () => {
	// ADR 0008: nothing is fetched until someone has actually asked for
	// something. Landing on /applications from the nav bar must not fan a
	// label-less list request out to every configured Argo CD.
	describe('with nothing submitted', () => {
		it('does not call the API at all', () => {
			const fetch = mockFetch(200, { results: [] });

			const { applications } = callLoad('http://localhost/applications/', fetch);

			expect(applications).toBeUndefined();
			expect(fetch).not.toHaveBeenCalled();
		});

		it('still hands the page an empty query to render its editor from', () => {
			const { query } = callLoad('http://localhost/applications/', mockFetch(200, {}));

			expect(query).toEqual({ labels: '', excludeLabels: '', targetRef: '' });
		});
	});

	// "Show me everything" is a legitimate thing to ask for, and the gate has
	// to let it through — otherwise submitting the editor with no labels
	// navigates to a URL that reads as "nothing asked for" and just reopens
	// the editor, with no way ever to see the whole fleet.
	describe('with an explicitly submitted empty query', () => {
		it('calls the API with no label filters', async () => {
			const fetch = mockFetch(200, { results: [] });

			const { applications } = callLoad('http://localhost/applications/?searched=true', fetch);
			await applications;

			expect(fetch).toHaveBeenCalledTimes(1);
			const [url] = fetch.mock.calls[0];
			expect(url).not.toContain('labels');
		});
	});

	it('fetches as soon as either label parameter is present', async () => {
		const fetch = mockFetch(200, { results: [] });

		const { applications } = callLoad(
			'http://localhost/applications/?excludeLabels=tier:sandbox',
			fetch
		);
		await applications;

		expect(fetch).toHaveBeenCalledTimes(1);
	});

	it('forwards labels and excludeLabels from the URL to the query string', async () => {
		const fetch = mockFetch(200, { results: [] });

		const { applications } = callLoad(
			'http://localhost/applications/?labels=foo:bar&excludeLabels=baz:qux',
			fetch
		);
		await applications;

		const [url] = fetch.mock.calls[0];
		expect(url).toContain('labels=foo%3Abar');
		expect(url).toContain('excludeLabels=baz%3Aqux');
	});

	it('passes the query through for the page header to render', () => {
		const { query } = callLoad(
			'http://localhost/applications/?labels=foo:bar&excludeLabels=baz:qux',
			mockFetch(200, {})
		);

		expect(query).toEqual({ labels: 'foo:bar', excludeLabels: 'baz:qux', targetRef: '' });
	});

	it('resolves to the parsed applications response', async () => {
		const fetch = mockFetch(200, { results: [{ name: 'test', link: '', applications: [] }] });

		const { applications } = callLoad('http://localhost/applications/?labels=foo:bar', fetch);
		const result = await applications;

		expect(result?.error).toBe(false);
		expect(result?.response.results).toHaveLength(1);
	});

	it('uses the fetch passed by SvelteKit, not the global one', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('global fetch should not be called'))
		);
		const injectedFetch = mockFetch(200, { results: [] });

		const { applications } = callLoad(
			'http://localhost/applications/?labels=foo:bar',
			injectedFetch
		);
		const result = await applications;

		expect(injectedFetch).toHaveBeenCalledTimes(1);
		expect(result?.error).toBe(false);

		vi.unstubAllGlobals();
	});
});

import { describe, it, expect, vi } from 'vitest';
import { load } from './+page';
import { type ApplicationResponseStore } from '$lib/backend/data';

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
	return load(makeEvent(url, fetch)) as { applications: Promise<ApplicationResponseStore> };
}

describe('applications +page.ts load', () => {
	it('requests applications with no query string when the URL has none', async () => {
		const fetch = mockFetch(200, { results: [] });

		const { applications } = callLoad('http://localhost/applications/', fetch);
		await applications;

		expect(fetch).toHaveBeenCalledTimes(1);
		const [url] = fetch.mock.calls[0];
		expect(url.endsWith('/api/applications')).toBe(true);
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

	it('resolves to the parsed applications response', async () => {
		const fetch = mockFetch(200, { results: [{ name: 'test', link: '', applications: [] }] });

		const { applications } = callLoad('http://localhost/applications/', fetch);
		const result = await applications;

		expect(result.error).toBe(false);
		expect(result.response.results).toHaveLength(1);
	});

	it('uses the fetch passed by SvelteKit, not the global one', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('global fetch should not be called'))
		);
		const injectedFetch = mockFetch(200, { results: [] });

		const { applications } = callLoad('http://localhost/applications/', injectedFetch);
		const result = await applications;

		expect(injectedFetch).toHaveBeenCalledTimes(1);
		expect(result.error).toBe(false);

		vi.unstubAllGlobals();
	});
});

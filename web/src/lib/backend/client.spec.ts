import { describe, it, expect, vi, afterEach } from 'vitest';
import TangleAPIClient from '$lib/backend/client';

function mockFetch(status: number, body: unknown) {
	const fetchMock = vi.fn().mockResolvedValue({
		status,
		json: () => Promise.resolve(body)
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

describe('TangleAPIClient', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	describe('getApplications', () => {
		it('requests the applications path with no query string when labels are absent', async () => {
			const fetchMock = mockFetch(200, { results: [] });
			const client = new TangleAPIClient();

			await client.getApplications(null, null);

			expect(fetchMock).toHaveBeenCalledTimes(1);
			const [url] = fetchMock.mock.calls[0];
			expect(url.endsWith('/api/applications')).toBe(true);
		});

		it('includes labels and excludeLabels in the query string', async () => {
			const fetchMock = mockFetch(200, { results: [] });
			const client = new TangleAPIClient();

			await client.getApplications('foo:bar', 'baz:qux');

			const [url] = fetchMock.mock.calls[0];
			expect(url).toContain('/api/applications?');
			expect(url).toContain('labels=foo%3Abar');
			expect(url).toContain('excludeLabels=baz%3Aqux');
		});

		it('returns the parsed applications on success', async () => {
			mockFetch(200, { results: [{ name: 'test', link: '', applications: [] }] });
			const client = new TangleAPIClient();

			const result = await client.getApplications(null, null);

			expect(result.error).toBe(false);
			expect(result.response.results).toHaveLength(1);
		});

		it('uses an injected fetch implementation instead of the global one', async () => {
			mockFetch(500, { error: 'global fetch should not be called' });
			const injectedFetch = vi.fn().mockResolvedValue({
				status: 200,
				json: () => Promise.resolve({ results: [] })
			});
			const client = new TangleAPIClient(injectedFetch);

			const result = await client.getApplications(null, null);

			expect(injectedFetch).toHaveBeenCalledTimes(1);
			expect(result.error).toBe(false);
		});
	});

	describe('getApplicationDiff', () => {
		it('POSTs the live/target refs to the argoCD diff endpoint', async () => {
			const fetchMock = mockFetch(200, {
				liveManifests: '',
				targetManifests: '',
				diffs: '',
				manifestGenerationError: ''
			});
			const client = new TangleAPIClient();

			await client.getApplicationDiff('test', 'my-app', 'main', 'feature');

			expect(fetchMock).toHaveBeenCalledTimes(1);
			const [url, options] = fetchMock.mock.calls[0];
			expect(url.endsWith('/api/argocd/test/applications/my-app/diffs')).toBe(true);
			expect(options.method).toBe('POST');
			expect(options.headers).toEqual({ 'Content-Type': 'application/json' });
			expect(JSON.parse(options.body)).toEqual({ liveRef: 'main', targetRef: 'feature' });
		});

		it('attaches requestDetails on success', async () => {
			mockFetch(200, {
				liveManifests: '',
				targetManifests: '',
				diffs: '',
				manifestGenerationError: ''
			});
			const client = new TangleAPIClient();

			const result = await client.getApplicationDiff('test', 'my-app', 'main', 'feature');

			expect(result.requestDetails).toEqual({ argoCD: 'test', applicationName: 'my-app' });
			expect(result.error).toBe(false);
		});

		it('attaches requestDetails on error too', async () => {
			mockFetch(500, { error: 'boom' });
			const client = new TangleAPIClient();

			const result = await client.getApplicationDiff('test', 'my-app', 'main', 'feature');

			expect(result.requestDetails).toEqual({ argoCD: 'test', applicationName: 'my-app' });
			expect(result.error).toBe(true);
			expect(result.errorResponse).toEqual({ error: 'boom' });
		});
	});
});

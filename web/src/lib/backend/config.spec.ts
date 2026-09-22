import { describe, it, expect, vi } from 'vitest';
import { fetchConfig } from '$lib/backend/config';

function mockFetch(status: number, body: unknown) {
	return vi.fn().mockResolvedValue({
		ok: status >= 200 && status < 300,
		status,
		json: () => Promise.resolve(body)
	});
}

describe('fetchConfig', () => {
	it('reads the domain the server reports', async () => {
		const fetch = mockFetch(200, { domain: 'https://tangle.corp' });

		expect(await fetchConfig(fetch)).toEqual({ domain: 'https://tangle.corp' });
	});

	it('requests the config endpoint', async () => {
		const fetch = mockFetch(200, { domain: '' });
		await fetchConfig(fetch);

		expect(fetch.mock.calls[0][0]).toContain('/api/config');
	});

	// Everything this config affects has a fallback, so a failure must not stop
	// the app rendering. An older server has no such endpoint at all.
	describe('degrading', () => {
		it('yields an empty domain on a non-OK response', async () => {
			expect(await fetchConfig(mockFetch(404, {}))).toEqual({ domain: '' });
		});

		it('yields an empty domain when the request throws', async () => {
			const fetch = vi.fn().mockRejectedValue(new Error('offline'));

			expect(await fetchConfig(fetch)).toEqual({ domain: '' });
		});

		it('yields an empty domain when the body is not the expected shape', async () => {
			expect(await fetchConfig(mockFetch(200, { domain: 42 }))).toEqual({ domain: '' });
			expect(await fetchConfig(mockFetch(200, {}))).toEqual({ domain: '' });
		});
	});
});

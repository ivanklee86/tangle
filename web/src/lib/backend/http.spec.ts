import { describe, it, expect, vi, afterEach } from 'vitest';
import { fetchEnvelope } from '$lib/backend/http';

interface Widget {
	name: string;
}

const emptyWidget: Widget = { name: '' };

describe('fetchEnvelope', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('returns the parsed response on a 200', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				status: 200,
				json: () => Promise.resolve({ name: 'foo' })
			})
		);

		const result = await fetchEnvelope<Widget>('/widgets', emptyWidget);

		expect(result).toEqual({
			response: { name: 'foo' },
			errorResponse: { error: '' },
			error: false,
			loaded: true
		});
	});

	it('returns the empty response and server error body on a non-200', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				status: 500,
				json: () => Promise.resolve({ error: 'boom' })
			})
		);

		const result = await fetchEnvelope<Widget>('/widgets', emptyWidget);

		expect(result).toEqual({
			response: emptyWidget,
			errorResponse: { error: 'boom' },
			error: true,
			loaded: true
		});
	});

	it('turns a thrown Error into an errorResponse message', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));

		const result = await fetchEnvelope<Widget>('/widgets', emptyWidget);

		expect(result).toEqual({
			response: emptyWidget,
			errorResponse: { error: 'network down' },
			error: true,
			loaded: true
		});
	});

	it('stringifies a thrown non-Error value', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue('nope'));

		const result = await fetchEnvelope<Widget>('/widgets', emptyWidget);

		expect(result.errorResponse.error).toBe('nope');
		expect(result.error).toBe(true);
	});

	it('uses an injected fetch implementation instead of the global one', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockRejectedValue(new Error('global fetch should not be called'))
		);
		const injectedFetch = vi.fn().mockResolvedValue({
			status: 200,
			json: () => Promise.resolve({ name: 'bar' })
		});

		const result = await fetchEnvelope<Widget>('/widgets', emptyWidget, undefined, injectedFetch);

		expect(injectedFetch).toHaveBeenCalledTimes(1);
		expect(result.response).toEqual({ name: 'bar' });
	});
});

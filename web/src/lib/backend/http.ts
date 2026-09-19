import { type TangleError } from '$lib/backend/data';

interface Envelope<T> {
	response: T;
	errorResponse: TangleError;
	error: boolean;
	loaded: boolean;
}

async function fetchEnvelope<T>(
	url: string,
	emptyResponse: T,
	options?: RequestInit,
	fetchImpl: typeof fetch = fetch
): Promise<Envelope<T>> {
	try {
		const response = await fetchImpl(url, options);
		const text = await response.text();

		if (response.status !== 200) {
			return {
				response: emptyResponse,
				errorResponse: parseErrorBody(text),
				error: true,
				loaded: true
			};
		}

		return {
			response: JSON.parse(text) as T,
			errorResponse: { error: '' },
			error: false,
			loaded: true
		};
	} catch (error) {
		return {
			response: emptyResponse,
			errorResponse: { error: error instanceof Error ? error.message : String(error) },
			error: true,
			loaded: true
		};
	}
}

function parseErrorBody(text: string): TangleError {
	try {
		return JSON.parse(text) as TangleError;
	} catch {
		return { error: text };
	}
}

export { fetchEnvelope, type Envelope };

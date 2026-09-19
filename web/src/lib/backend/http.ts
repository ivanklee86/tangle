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
		const data = await response.json();

		if (response.status !== 200) {
			return {
				response: emptyResponse,
				errorResponse: data as TangleError,
				error: true,
				loaded: true
			};
		}

		return { response: data as T, errorResponse: { error: '' }, error: false, loaded: true };
	} catch (error) {
		return {
			response: emptyResponse,
			errorResponse: { error: error instanceof Error ? error.message : String(error) },
			error: true,
			loaded: true
		};
	}
}

export { fetchEnvelope, type Envelope };

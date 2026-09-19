import TangleAPIClient from '$lib/backend/client';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const client = new TangleAPIClient(fetch);

	// Deliberately not awaited — streamed, so the page can show a loading
	// state via {#await} while it resolves, matching the previous onMount
	// + spinner behavior instead of blocking navigation on the fetch.
	return {
		applications: client.getApplications(
			url.searchParams.get('labels'),
			url.searchParams.get('excludeLabels')
		)
	};
};

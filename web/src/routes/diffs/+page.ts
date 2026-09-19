import TangleAPIClient from '$lib/backend/client';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const client = new TangleAPIClient(fetch);

	// Deliberately not awaited — streamed, consumed via {#await} in the page.
	// Diff fan-out (progress bar) stays client-side, triggered off the
	// resolved value, not folded into this load() itself.
	return {
		applications: client.getApplications(
			url.searchParams.get('labels'),
			url.searchParams.get('excludeLabels')
		)
	};
};

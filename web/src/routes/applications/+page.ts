import TangleAPIClient from '$lib/backend/client';
import { hasSubmittedQuery, queryFromParams } from '$lib/ui/query';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const query = queryFromParams(url.searchParams);

	// A bare /applications means nobody has asked for anything yet — the page
	// opens its editor instead (ADR 0008: don't fan out across every Argo CD
	// because someone clicked a nav link). A *submitted* query with no labels
	// is a different thing: it means "show me everything", and it runs.
	if (!hasSubmittedQuery(url.searchParams)) {
		return { query, applications: undefined };
	}

	const client = new TangleAPIClient(fetch);

	// Deliberately not awaited — streamed, so the page can render its header
	// and toolbar from the URL while the fetch resolves.
	return {
		query,
		applications: client.getApplications(query.labels, query.excludeLabels)
	};
};

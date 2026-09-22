import TangleAPIClient from '$lib/backend/client';
import { isEmptyQuery, queryFromParams } from '$lib/ui/query';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const query = queryFromParams(url.searchParams);

	// No query means nobody has asked for anything yet — the page opens its
	// editor instead (ADR 0008: don't fan out across every Argo CD because
	// someone clicked a nav link).
	if (isEmptyQuery(query)) {
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

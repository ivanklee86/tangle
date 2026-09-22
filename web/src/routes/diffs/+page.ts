import TangleAPIClient from '$lib/backend/client';
import { isEmptyQuery, queryFromParams } from '$lib/ui/query';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const query = queryFromParams(url.searchParams);

	// No query, or no ref to diff against, means nobody has asked for anything
	// yet — the page opens its editor instead. ADR 0008: a bare visit must not
	// fan a diff-generation request per application out to every Argo CD.
	if (isEmptyQuery(query) || query.targetRef.length === 0) {
		return { query, applications: undefined };
	}

	const client = new TangleAPIClient(fetch);

	// Deliberately not awaited — streamed, so the page can render its header
	// and sidebar shell while the fetch resolves. The diff fan-out stays
	// client-side, triggered off the resolved value rather than folded in here.
	return {
		query,
		applications: client.getApplications(query.labels, query.excludeLabels)
	};
};

import TangleAPIClient from '$lib/backend/client';
import { queryFromParams } from '$lib/ui/query';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const query = queryFromParams(url.searchParams);

	// The target ref is this page's gate: there is nothing to compare against
	// without one, and a bare visit has none — so ADR 0008's concern (a nav
	// click fanning one diff-generation POST per application out to every Argo
	// CD) is covered without also requiring labels. A ref with no labels is a
	// real request: diff everything.
	if (query.targetRef.length === 0) {
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

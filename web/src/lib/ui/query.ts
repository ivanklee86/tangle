import { buildQuery } from '$lib/backend/url';
import { parseLabels } from '$lib/ui/labels';

interface Query {
	labels: string;
	excludeLabels: string;
	targetRef: string;
}

function emptyQuery(): Query {
	return { labels: '', excludeLabels: '', targetRef: '' };
}

/** Reads the query a page was opened with straight back out of its URL. */
function queryFromParams(params: URLSearchParams): Query {
	return {
		labels: params.get('labels') ?? '',
		excludeLabels: params.get('excludeLabels') ?? '',
		targetRef: params.get('targetRef') ?? ''
	};
}

/** True when the query carries no label filters of either kind. */
function isEmptyQuery(query: Query): boolean {
	return query.labels.length === 0 && query.excludeLabels.length === 0;
}

/**
 * Marks a URL as carrying a query somebody actually submitted.
 *
 * "No labels" is a legitimate query — it means "show me everything" — but an
 * empty label filter drops out of the query string entirely, so a submitted
 * empty query and a bare nav click produce the same URL. This flag is what
 * tells them apart, and without it ADR 0008's gate has no way to let an empty
 * submit through.
 */
const SEARCHED_PARAM = 'searched';

/**
 * Whether the page should run the query in this URL, or open its editor.
 *
 * Any label filter counts, and so does the explicit marker above. A bare
 * /applications does not — that's a nav click, and ADR 0008 says it must not
 * fan out across every configured Argo CD.
 */
function hasSubmittedQuery(params: URLSearchParams): boolean {
	return (
		(params.get('labels') ?? '').length > 0 ||
		(params.get('excludeLabels') ?? '').length > 0 ||
		params.get(SEARCHED_PARAM) === 'true'
	);
}

function applicationsHref(query: Query): string {
	return `/applications${buildQuery({
		labels: query.labels,
		excludeLabels: query.excludeLabels,
		// Only when there is nothing else to carry: with labels present the
		// URL already says a query was submitted, and the extra parameter
		// would just be noise in a link people copy and share.
		[SEARCHED_PARAM]: isEmptyQuery(query) ? 'true' : ''
	})}`;
}

function diffsHref(query: Query): string {
	// No marker needed here: diffs can't run without a target ref, and that
	// ref is always in the URL, so "submitted" and "bare visit" already look
	// different.
	return `/diffs${buildQuery({
		targetRef: query.targetRef,
		labels: query.labels,
		excludeLabels: query.excludeLabels
	})}`;
}

/**
 * The same query as a tangle-cli invocation, so someone who built a query in
 * the browser can paste it into CI rather than re-deriving the flags.
 *
 * The CLI takes `key=value` per flag while the API takes `key:value` in one
 * comma-separated parameter, so this is a translation, not a reformat — flag
 * names match cmd/tangle-cli/main.go.
 */
function cliCommand(query: Query): string {
	const parts = ['tangle-cli generate-manifests'];

	for (const { key, value } of parseLabels(query.labels)) {
		parts.push(`--label ${key}=${value}`);
	}
	for (const { key, value } of parseLabels(query.excludeLabels)) {
		parts.push(`--exclude-label ${key}=${value}`);
	}
	if (query.targetRef.length > 0) {
		parts.push(`--target-ref ${query.targetRef}`);
	}

	return parts.join(' ');
}

export {
	applicationsHref,
	cliCommand,
	diffsHref,
	emptyQuery,
	hasSubmittedQuery,
	isEmptyQuery,
	queryFromParams,
	SEARCHED_PARAM,
	type Query
};

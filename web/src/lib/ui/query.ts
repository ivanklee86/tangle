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

/** True when nothing has been asked for — the state that opens the editor. */
function isEmptyQuery(query: Query): boolean {
	return query.labels.length === 0 && query.excludeLabels.length === 0;
}

function applicationsHref(query: Query): string {
	return `/applications${buildQuery({
		labels: query.labels,
		excludeLabels: query.excludeLabels
	})}`;
}

function diffsHref(query: Query): string {
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
	isEmptyQuery,
	queryFromParams,
	type Query
};

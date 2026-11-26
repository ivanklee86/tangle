import {
	type ApplicationDiff,
	type ApplicationSource,
	type ArgoCDApplicationResults
} from '$lib/backend/data';

/** Where one application's diff has got to, and what it found. */
type DiffOutcome =
	| 'pending'
	| 'error'
	| 'changed'
	| 'unchanged'
	// Target repos were given and none of the app's sources use them: nothing
	// was rendered, and it isn't a failure.
	| 'notAffected'
	// The app has several sources and the query didn't say which repo the ref
	// belongs to. CI fails on it (ADR 0032, D2).
	| 'needsTargetRepo';

/** Which applications the sidebar is showing. */
type DiffFilter = 'changed' | 'needsRepo' | 'errors' | 'all';

interface DiffStats {
	added: number;
	removed: number;
}

interface DiffRow {
	instance: string;
	name: string;
	url: string;
	health: string;
	syncStatus: string;
	liveRef: string;
	/** Every source, in ArgoCD's order. Empty from a server that predates them. */
	sources: ApplicationSource[];
	outcome: DiffOutcome;
	stats: DiffStats;
	diff: ApplicationDiff | undefined;
}

/**
 * Counts changed lines in a unified diff.
 *
 * `---` and `+++` are the file headers, not content, so they're excluded —
 * otherwise every diff, however small, reports one extra addition and removal.
 * Headers only come before the first `@@` hunk, though: inside a hunk a
 * removed YAML document separator reads `----`, and that is content.
 */
function diffStats(diffText: string): DiffStats {
	let added = 0;
	let removed = 0;
	let inHunk = false;

	for (const line of diffText.split('\n')) {
		if (line.startsWith('@@')) inHunk = true;
		else if (!inHunk && (line.startsWith('---') || line.startsWith('+++'))) continue;
		else if (line.startsWith('+')) added += 1;
		else if (line.startsWith('-')) removed += 1;
	}

	return { added, removed };
}

/**
 * Classifies one application's diff.
 *
 * A missing entry means the fan-out hasn't reached it yet — the sidebar lists
 * every application as soon as the applications call returns, so rows exist
 * before their diffs do. A manifest-generation error counts as an error even
 * though the request itself succeeded: the caller asked for a diff and didn't
 * get one.
 *
 * The server's status comes first. "Needs target repo" also sets
 * manifestGenerationError, so CLIs that predate the status still fail on it,
 * but this page can say what's actually wrong.
 */
function diffOutcome(diff: ApplicationDiff | undefined): DiffOutcome {
	if (!diff || !diff.loaded) return 'pending';
	if (!diff.error && diff.response.status === 'needsTargetRepo') return 'needsTargetRepo';
	if (!diff.error && diff.response.status === 'notAffected') return 'notAffected';
	if (diff.error || diff.response.manifestGenerationError.length > 0) return 'error';
	return diff.response.diffs.length > 0 ? 'changed' : 'unchanged';
}

/** One row per application, in the order the instances answered. */
function buildDiffRows(
	results: ArgoCDApplicationResults[],
	diffs: Record<string, Record<string, ApplicationDiff>>
): DiffRow[] {
	return results.flatMap((result) =>
		result.applications.map((application) => {
			const diff = diffs[result.name]?.[application.name];
			return {
				instance: result.name,
				name: application.name,
				url: application.url,
				health: application.health,
				syncStatus: application.syncStatus,
				liveRef: application.liveRef,
				sources: application.sources ?? [],
				outcome: diffOutcome(diff),
				stats: diff?.loaded ? diffStats(diff.response.diffs) : { added: 0, removed: 0 },
				diff
			};
		})
	);
}

function countOutcome(rows: DiffRow[], outcome: DiffOutcome): number {
	return rows.filter((row) => row.outcome === outcome).length;
}

/** How many diffs have come back, for the progress footer. */
function loadedCount(rows: DiffRow[]): number {
	return rows.filter((row) => row.outcome !== 'pending').length;
}

/**
 * The rows the list can show at all: everything, or everything but "not
 * affected" (D10). Those are hidden by default, because with target repos set
 * they're often most of the fleet and none of them needs a look. The outcome
 * segments count from this, so "All" is the number of rows actually listed.
 */
function listableRows(rows: DiffRow[], showNotAffected: boolean): DiffRow[] {
	return showNotAffected ? rows : rows.filter((row) => row.outcome !== 'notAffected');
}

function filterDiffRows(rows: DiffRow[], filter: DiffFilter, name: string): DiffRow[] {
	const needle = name.trim().toLowerCase();

	return rows.filter((row) => {
		if (filter === 'changed' && row.outcome !== 'changed') return false;
		if (filter === 'needsRepo' && row.outcome !== 'needsTargetRepo') return false;
		if (filter === 'errors' && row.outcome !== 'error') return false;
		if (needle.length > 0 && !row.name.toLowerCase().includes(needle)) return false;
		return true;
	});
}

/**
 * Groups rows under their Argo CD, preserving the order instances appear in.
 * The sidebar needs the grouping to render a heading per instance, and losing
 * the order would reshuffle the list as diffs arrive.
 */
function groupByInstance(rows: DiffRow[]): { instance: string; rows: DiffRow[] }[] {
	const groups: { instance: string; rows: DiffRow[] }[] = [];

	for (const row of rows) {
		const existing = groups.find((group) => group.instance === row.instance);
		if (existing) existing.rows.push(row);
		else groups.push({ instance: row.instance, rows: [row] });
	}

	return groups;
}

/** The row a page should show when nothing is selected, or the selection is gone. */
function resolveSelection(rows: DiffRow[], selected: string | undefined): DiffRow | undefined {
	return rows.find((row) => rowKey(row) === selected) ?? rows[0];
}

/** Stable identity for a row — application names are only unique per instance. */
function rowKey(row: DiffRow): string {
	return `${row.instance}/${row.name}`;
}

export {
	buildDiffRows,
	countOutcome,
	diffOutcome,
	diffStats,
	filterDiffRows,
	groupByInstance,
	listableRows,
	loadedCount,
	resolveSelection,
	rowKey,
	type DiffFilter,
	type DiffOutcome,
	type DiffRow,
	type DiffStats
};

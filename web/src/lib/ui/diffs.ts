import { type ApplicationDiff, type ArgoCDApplicationResults } from '$lib/backend/data';

/** Where one application's diff has got to, and what it found. */
type DiffOutcome = 'pending' | 'error' | 'changed' | 'unchanged';

/** Which applications the sidebar is showing. */
type DiffFilter = 'changed' | 'errors' | 'all';

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
	outcome: DiffOutcome;
	stats: DiffStats;
	diff: ApplicationDiff | undefined;
}

/**
 * Counts changed lines in a unified diff.
 *
 * `---` and `+++` are the file headers, not content, so they're excluded —
 * otherwise every diff, however small, reports one extra addition and removal.
 */
function diffStats(diffText: string): DiffStats {
	let added = 0;
	let removed = 0;

	for (const line of diffText.split('\n')) {
		if (line.startsWith('+') && !line.startsWith('+++')) added += 1;
		else if (line.startsWith('-') && !line.startsWith('---')) removed += 1;
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
 */
function diffOutcome(diff: ApplicationDiff | undefined): DiffOutcome {
	if (!diff || !diff.loaded) return 'pending';
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

function filterDiffRows(rows: DiffRow[], filter: DiffFilter, name: string): DiffRow[] {
	const needle = name.trim().toLowerCase();

	return rows.filter((row) => {
		if (filter === 'changed' && row.outcome !== 'changed') return false;
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
	loadedCount,
	resolveSelection,
	rowKey,
	type DiffFilter,
	type DiffOutcome,
	type DiffRow,
	type DiffStats
};

import { type ApplicationLinks, type ArgoCDApplicationResults } from '$lib/backend/data';
import { needsAttention } from '$lib/ui/status';

/** One application, plus which Argo CD it came from. */
interface ApplicationRow extends ApplicationLinks {
	instance: string;
}

interface Facets {
	/** Restrict to applications worth a second look. */
	attentionOnly: boolean;
	/** Health values to keep. Empty means "any". */
	health: Set<string>;
	/** Sync values to keep. Empty means "any". */
	sync: Set<string>;
	/** Case-insensitive substring match on the application name. */
	name: string;
}

interface FacetCount {
	value: string;
	count: number;
}

function emptyFacets(): Facets {
	return { attentionOnly: false, health: new Set(), sync: new Set(), name: '' };
}

/**
 * Flattens the per-instance response into one list.
 *
 * The page shows a single table with an Argo CD column rather than a tab per
 * instance: the question people arrive with is "what in my fleet needs
 * attention", and a tab strip makes that answerable only one instance at a
 * time.
 */
function flattenApplications(results: ArgoCDApplicationResults[]): ApplicationRow[] {
	return results.flatMap((result) =>
		result.applications.map((application) => ({ ...application, instance: result.name }))
	);
}

/** Distinct values of one field with their counts, most common first. */
function countBy(rows: ApplicationRow[], field: 'health' | 'syncStatus'): FacetCount[] {
	const counts = new Map<string, number>();
	for (const row of rows) {
		counts.set(row[field], (counts.get(row[field]) ?? 0) + 1);
	}

	return [...counts.entries()]
		.map(([value, count]) => ({ value, count }))
		.sort((a, b) => b.count - a.count || a.value.localeCompare(b.value));
}

function attentionCount(rows: ApplicationRow[]): number {
	return rows.filter((row) => needsAttention(row.health, row.syncStatus)).length;
}

/**
 * Applies the toolbar facets. Each facet narrows the last, and an empty set
 * means the facet isn't filtering at all — so "no selection" shows everything
 * rather than nothing.
 */
function filterApplications(rows: ApplicationRow[], facets: Facets): ApplicationRow[] {
	const name = facets.name.trim().toLowerCase();

	return rows.filter((row) => {
		if (facets.attentionOnly && !needsAttention(row.health, row.syncStatus)) return false;
		if (facets.health.size > 0 && !facets.health.has(row.health)) return false;
		if (facets.sync.size > 0 && !facets.sync.has(row.syncStatus)) return false;
		if (name.length > 0 && !row.name.toLowerCase().includes(name)) return false;
		return true;
	});
}

/** Toggles one value in a facet set, returning a new set so $state sees it. */
function toggleFacet(current: Set<string>, value: string): Set<string> {
	const next = new Set(current);
	if (!next.delete(value)) next.add(value);
	return next;
}

function hasActiveFacets(facets: Facets): boolean {
	return (
		facets.attentionOnly ||
		facets.health.size > 0 ||
		facets.sync.size > 0 ||
		facets.name.trim().length > 0
	);
}

export {
	attentionCount,
	countBy,
	emptyFacets,
	filterApplications,
	flattenApplications,
	hasActiveFacets,
	toggleFacet,
	type ApplicationRow,
	type FacetCount,
	type Facets
};

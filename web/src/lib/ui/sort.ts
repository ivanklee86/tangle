import { type ApplicationLinks } from '$lib/backend/data';
import { statusSeverity } from '$lib/ui/status';

type SortKey = 'name' | 'health' | 'syncStatus';
type SortDirection = 'asc' | 'desc';
type SortState = { key: SortKey; direction: SortDirection };

function nextSortState(current: SortState | undefined, key: SortKey): SortState {
	if (current?.key === key) {
		return { key, direction: current.direction === 'asc' ? 'desc' : 'asc' };
	}
	return { key, direction: 'asc' };
}

function sortIndicator(state: SortState | undefined, key: SortKey): string {
	if (state?.key !== key) return '';
	return state.direction === 'asc' ? ' ▲' : ' ▼';
}

function ariaSort(state: SortState | undefined, key: SortKey): 'ascending' | 'descending' | 'none' {
	if (state?.key !== key) return 'none';
	return state.direction === 'asc' ? 'ascending' : 'descending';
}

/**
 * Compares two applications on one column.
 *
 * Status columns compare on severity, not on the status string: sorting health
 * alphabetically gives Degraded, Healthy, Missing, Progressing, which is not
 * what anyone means by sorting a health column. Ties inside a severity bucket
 * fall back to the name, so Degraded rows keep a stable, readable order among
 * themselves rather than whatever order the instances answered in.
 */
function compareApplications(a: ApplicationLinks, b: ApplicationLinks, key: SortKey): number {
	if (key === 'name') return a.name.localeCompare(b.name);

	const severity = statusSeverity(a[key]) - statusSeverity(b[key]);
	return severity !== 0 ? severity : a.name.localeCompare(b.name);
}

function sortApplications(
	applications: ApplicationLinks[],
	state: SortState | undefined
): ApplicationLinks[] {
	if (!state) return applications;

	// Sort ascending, then reverse for descending — rather than negating the
	// comparator — so the name tiebreak reverses with it and the result stays
	// a total order either way.
	const sorted = [...applications].sort((a, b) => compareApplications(a, b, state.key));
	return state.direction === 'asc' ? sorted : sorted.reverse();
}

export { nextSortState, sortIndicator, ariaSort, sortApplications, type SortKey, type SortState };

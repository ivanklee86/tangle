import { type ApplicationLinks } from '$lib/backend/data';

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

function sortApplications(
	applications: ApplicationLinks[],
	state: SortState | undefined
): ApplicationLinks[] {
	if (!state) return applications;
	const sorted = [...applications].sort((a, b) => a[state.key].localeCompare(b[state.key]));
	return state.direction === 'asc' ? sorted : sorted.reverse();
}

export { nextSortState, sortIndicator, ariaSort, sortApplications, type SortKey, type SortState };

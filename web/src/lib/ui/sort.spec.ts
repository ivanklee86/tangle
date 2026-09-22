import { describe, it, expect } from 'vitest';
import {
	nextSortState,
	sortIndicator,
	ariaSort,
	sortApplications,
	type SortState
} from '$lib/ui/sort';
import { type ApplicationLinks } from '$lib/backend/data';

function makeApps(): ApplicationLinks[] {
	return [
		{ name: 'charlie', url: '', health: 'Degraded', syncStatus: 'OutOfSync', liveRef: 'main' },
		{ name: 'alpha', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' },
		{ name: 'bravo', url: '', health: 'Missing', syncStatus: 'Unknown', liveRef: 'main' }
	];
}

describe('nextSortState', () => {
	it('defaults to ascending when there is no prior state', () => {
		expect(nextSortState(undefined, 'name')).toEqual({ key: 'name', direction: 'asc' });
	});

	it('toggles direction on the same key', () => {
		const asc: SortState = { key: 'name', direction: 'asc' };
		expect(nextSortState(asc, 'name')).toEqual({ key: 'name', direction: 'desc' });

		const desc: SortState = { key: 'name', direction: 'desc' };
		expect(nextSortState(desc, 'name')).toEqual({ key: 'name', direction: 'asc' });
	});

	it('resets to ascending when a different key is chosen', () => {
		const current: SortState = { key: 'name', direction: 'desc' };
		expect(nextSortState(current, 'health')).toEqual({ key: 'health', direction: 'asc' });
	});
});

describe('sortApplications', () => {
	it('returns the input unchanged when there is no sort state', () => {
		const apps = makeApps();
		expect(sortApplications(apps, undefined)).toBe(apps);
	});

	it('sorts ascending by name without mutating the input', () => {
		const apps = makeApps();
		const sorted = sortApplications(apps, { key: 'name', direction: 'asc' });

		expect(sorted.map((a) => a.name)).toEqual(['alpha', 'bravo', 'charlie']);
		expect(apps.map((a) => a.name)).toEqual(['charlie', 'alpha', 'bravo']);
	});

	// Status columns sort on severity, not alphabetically. Sorting health by
	// its string gives Degraded, Healthy, Missing, Progressing — which is not
	// what anyone clicking a health column means, and would make the
	// "worst first" default order in the table footer a lie.
	it('sorts health worst-first when descending', () => {
		const sorted = sortApplications(makeApps(), { key: 'health', direction: 'desc' });
		expect(sorted.map((a) => a.health)).toEqual(['Degraded', 'Missing', 'Healthy']);
	});

	it('sorts health best-first when ascending', () => {
		const sorted = sortApplications(makeApps(), { key: 'health', direction: 'asc' });
		expect(sorted.map((a) => a.health)).toEqual(['Healthy', 'Missing', 'Degraded']);
	});

	it('sorts syncStatus by severity, not alphabetically', () => {
		const sorted = sortApplications(makeApps(), { key: 'syncStatus', direction: 'asc' });
		expect(sorted.map((a) => a.syncStatus)).toEqual(['Synced', 'Unknown', 'OutOfSync']);
	});

	it('breaks severity ties by name, so equal statuses keep a stable order', () => {
		// Without a tiebreak the order inside a bucket is whatever order the
		// instances answered in, which changes under auto-refresh.
		const apps: ApplicationLinks[] = [
			{ name: 'zulu', url: '', health: 'Degraded', syncStatus: 'Synced', liveRef: 'main' },
			{ name: 'alpha', url: '', health: 'Missing', syncStatus: 'Synced', liveRef: 'main' },
			{ name: 'mike', url: '', health: 'Degraded', syncStatus: 'Synced', liveRef: 'main' }
		];

		const sorted = sortApplications(apps, { key: 'health', direction: 'asc' });
		expect(sorted.map((a) => a.name)).toEqual(['alpha', 'mike', 'zulu']);
	});
});

describe('sortIndicator', () => {
	it('is empty when the state is for a different key or unset', () => {
		expect(sortIndicator(undefined, 'name')).toBe('');
		expect(sortIndicator({ key: 'health', direction: 'asc' }, 'name')).toBe('');
	});

	it('shows the right glyph for the active key', () => {
		expect(sortIndicator({ key: 'name', direction: 'asc' }, 'name')).toBe(' ▲');
		expect(sortIndicator({ key: 'name', direction: 'desc' }, 'name')).toBe(' ▼');
	});
});

describe('ariaSort', () => {
	it('is "none" when the state is for a different key or unset', () => {
		expect(ariaSort(undefined, 'name')).toBe('none');
		expect(ariaSort({ key: 'health', direction: 'asc' }, 'name')).toBe('none');
	});

	it('maps direction to the right aria value for the active key', () => {
		expect(ariaSort({ key: 'name', direction: 'asc' }, 'name')).toBe('ascending');
		expect(ariaSort({ key: 'name', direction: 'desc' }, 'name')).toBe('descending');
	});
});

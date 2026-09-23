import { describe, it, expect } from 'vitest';
import {
	buildDiffRows,
	countOutcome,
	diffOutcome,
	diffStats,
	filterDiffRows,
	groupByInstance,
	loadedCount,
	resolveSelection,
	rowKey,
	type DiffRow
} from '$lib/ui/diffs';
import { type ApplicationDiff, type ArgoCDApplicationResults } from '$lib/backend/data';

function diff(
	overrides: Partial<ApplicationDiff['response']> = {},
	loaded = true
): ApplicationDiff {
	return {
		response: {
			liveManifests: '',
			targetManifests: '',
			diffs: '',
			manifestGenerationError: '',
			...overrides
		},
		errorResponse: { error: '' },
		requestDetails: { argoCD: 'test', applicationName: 'alpha' },
		error: false,
		loaded
	};
}

describe('diffStats', () => {
	it('counts added and removed lines', () => {
		expect(
			diffStats('--- live\n+++ target\n@@ -3,3 +3,3 @@\n-  color: blue\n+  color: green\n')
		).toEqual({ added: 1, removed: 1 });
	});

	// The `---`/`+++` header lines start with the same characters as content.
	// Counting them would make every diff, however small, report one extra
	// addition and one extra removal.
	it('ignores the file headers', () => {
		expect(diffStats('--- live\n+++ target\n')).toEqual({ added: 0, removed: 0 });
	});

	// Manifests are multi-document YAML, so removing a resource removes its
	// `---` separator — a content line that looks like a header. Only the
	// lines before the first hunk are headers.
	it('counts a removed document separator inside a hunk', () => {
		expect(diffStats('--- live\n+++ target\n@@ -1,3 +1,1 @@\n kind: A\n----\n-kind: B\n')).toEqual({
			added: 0,
			removed: 2
		});
	});

	it('counts an added line whose content starts with ++', () => {
		expect(diffStats('--- live\n+++ target\n@@ -1,1 +1,2 @@\n a\n+++b\n')).toEqual({
			added: 1,
			removed: 0
		});
	});

	it('is zero for an empty diff', () => {
		expect(diffStats('')).toEqual({ added: 0, removed: 0 });
	});

	it('ignores context and hunk lines', () => {
		expect(diffStats('@@ -1,2 +1,2 @@\n unchanged\n-gone\n+new\n')).toEqual({
			added: 1,
			removed: 1
		});
	});
});

describe('diffOutcome', () => {
	// The sidebar lists every application as soon as the applications call
	// returns, so rows exist before their diffs do.
	it('is pending when no diff has arrived', () => {
		expect(diffOutcome(undefined)).toBe('pending');
	});

	it('is pending while a diff is still in flight', () => {
		expect(diffOutcome(diff({}, false))).toBe('pending');
	});

	it('is changed when the diff has content', () => {
		expect(diffOutcome(diff({ diffs: '+ added\n' }))).toBe('changed');
	});

	it('is unchanged when the diff came back empty', () => {
		expect(diffOutcome(diff({ diffs: '' }))).toBe('unchanged');
	});

	it('is an error when the request failed', () => {
		expect(diffOutcome({ ...diff(), error: true })).toBe('error');
	});

	// The request succeeded but the caller still didn't get a diff, which is
	// the thing they asked for.
	it('is an error when manifests could not be generated', () => {
		expect(diffOutcome(diff({ manifestGenerationError: 'boom' }))).toBe('error');
	});
});

const RESULTS: ArgoCDApplicationResults[] = [
	{
		name: 'test',
		link: '',
		applications: [
			{ name: 'alpha', url: 'u/alpha', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' },
			{
				name: 'bravo',
				url: 'u/bravo',
				health: 'Degraded',
				syncStatus: 'OutOfSync',
				liveRef: 'main'
			}
		]
	},
	{
		name: 'prod',
		link: '',
		applications: [
			{
				name: 'charlie',
				url: 'u/charlie',
				health: 'Healthy',
				syncStatus: 'Synced',
				liveRef: 'main'
			}
		]
	}
];

function rows(): DiffRow[] {
	return buildDiffRows(RESULTS, {
		test: {
			alpha: diff({ diffs: '--- live\n+++ target\n+added\n-gone\n' }),
			bravo: diff({ manifestGenerationError: 'boom' })
		}
		// prod/charlie deliberately absent: still pending.
	});
}

describe('buildDiffRows', () => {
	it('produces one row per application, tagged with its instance', () => {
		expect(rows().map((r) => rowKey(r))).toEqual(['test/alpha', 'test/bravo', 'prod/charlie']);
	});

	it('classifies each row and carries its line counts', () => {
		const [alpha, bravo, charlie] = rows();

		expect(alpha.outcome).toBe('changed');
		expect(alpha.stats).toEqual({ added: 1, removed: 1 });

		expect(bravo.outcome).toBe('error');
		expect(charlie.outcome).toBe('pending');
	});

	it('reports no line counts for a row whose diff has not arrived', () => {
		expect(rows()[2].stats).toEqual({ added: 0, removed: 0 });
	});

	it('keeps the application details the sidebar and header need', () => {
		expect(rows()[0]).toMatchObject({
			instance: 'test',
			name: 'alpha',
			url: 'u/alpha',
			health: 'Healthy',
			liveRef: 'main'
		});
	});
});

describe('countOutcome and loadedCount', () => {
	it('counts each outcome', () => {
		expect(countOutcome(rows(), 'changed')).toBe(1);
		expect(countOutcome(rows(), 'error')).toBe(1);
		expect(countOutcome(rows(), 'pending')).toBe(1);
	});

	it('counts everything that is no longer pending, for the progress footer', () => {
		expect(loadedCount(rows())).toBe(2);
	});
});

describe('filterDiffRows', () => {
	it('shows everything under "all"', () => {
		expect(filterDiffRows(rows(), 'all', '')).toHaveLength(3);
	});

	it('narrows to changed applications', () => {
		expect(filterDiffRows(rows(), 'changed', '').map((r) => r.name)).toEqual(['alpha']);
	});

	it('narrows to errors', () => {
		expect(filterDiffRows(rows(), 'errors', '').map((r) => r.name)).toEqual(['bravo']);
	});

	it('matches names case-insensitively alongside the outcome filter', () => {
		expect(filterDiffRows(rows(), 'all', 'CHAR').map((r) => r.name)).toEqual(['charlie']);
		expect(filterDiffRows(rows(), 'changed', 'char')).toEqual([]);
	});
});

describe('groupByInstance', () => {
	it('groups rows under their Argo CD, keeping instance order', () => {
		expect(groupByInstance(rows()).map((g) => [g.instance, g.rows.length])).toEqual([
			['test', 2],
			['prod', 1]
		]);
	});

	it('returns nothing for no rows', () => {
		expect(groupByInstance([])).toEqual([]);
	});
});

describe('resolveSelection', () => {
	it('returns the selected row when it is still there', () => {
		expect(resolveSelection(rows(), 'test/bravo')?.name).toBe('bravo');
	});

	it('falls back to the first row when nothing is selected', () => {
		expect(resolveSelection(rows(), undefined)?.name).toBe('alpha');
	});

	// Filtering can remove whatever was selected; the detail pane has to show
	// something rather than going blank.
	it('falls back to the first row when the selection has been filtered away', () => {
		expect(resolveSelection(rows(), 'prod/nonexistent')?.name).toBe('alpha');
	});

	it('is undefined when there are no rows at all', () => {
		expect(resolveSelection([], 'test/alpha')).toBeUndefined();
	});
});

import { describe, it, expect } from 'vitest';
import {
	attentionCount,
	countBy,
	emptyFacets,
	filterApplications,
	flattenApplications,
	hasActiveFacets,
	toggleFacet,
	type ApplicationRow
} from '$lib/ui/applications';
import { type ArgoCDApplicationResults } from '$lib/backend/data';

function row(name: string, instance: string, health: string, syncStatus: string): ApplicationRow {
	return { name, instance, health, syncStatus, url: '', liveRef: 'main' };
}

function fleet(): ApplicationRow[] {
	return [
		row('payments-api', 'prod', 'Degraded', 'OutOfSync'),
		row('backend', 'test', 'Degraded', 'Synced'),
		row('cron-runner', 'test', 'Progressing', 'Synced'),
		row('frontend', 'test', 'Healthy', 'OutOfSync'),
		row('gateway', 'prod', 'Healthy', 'Synced')
	];
}

describe('flattenApplications', () => {
	it('tags each application with the instance it came from', () => {
		const results: ArgoCDApplicationResults[] = [
			{
				name: 'test',
				link: '',
				applications: [
					{ name: 'frontend', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
				]
			},
			{
				name: 'prod',
				link: '',
				applications: [
					{ name: 'gateway', url: '', health: 'Healthy', syncStatus: 'Synced', liveRef: 'main' }
				]
			}
		];

		expect(flattenApplications(results).map((r) => [r.name, r.instance])).toEqual([
			['frontend', 'test'],
			['gateway', 'prod']
		]);
	});

	it('returns nothing for a fleet with no applications', () => {
		expect(flattenApplications([])).toEqual([]);
		expect(flattenApplications([{ name: 'test', link: '', applications: [] }])).toEqual([]);
	});
});

describe('countBy', () => {
	it('counts distinct values, most common first', () => {
		expect(countBy(fleet(), 'health')).toEqual([
			{ value: 'Degraded', count: 2 },
			{ value: 'Healthy', count: 2 },
			{ value: 'Progressing', count: 1 }
		]);
	});

	it('breaks count ties alphabetically, so the toolbar does not reshuffle', () => {
		// Degraded and Healthy both have 2 above; a stable order matters
		// because these render as buttons people aim at.
		const values = countBy(fleet(), 'health').map((f) => f.value);
		expect(values.indexOf('Degraded')).toBeLessThan(values.indexOf('Healthy'));
	});
});

describe('attentionCount', () => {
	it('counts anything that is not both Healthy and Synced', () => {
		// payments-api, backend, cron-runner, and frontend (healthy but drifted).
		expect(attentionCount(fleet())).toBe(4);
	});

	it('is zero for a clean fleet', () => {
		expect(attentionCount([row('gateway', 'prod', 'Healthy', 'Synced')])).toBe(0);
	});
});

describe('filterApplications', () => {
	it('returns everything when no facet is set', () => {
		expect(filterApplications(fleet(), emptyFacets())).toHaveLength(5);
	});

	it('narrows to applications needing attention', () => {
		const filtered = filterApplications(fleet(), { ...emptyFacets(), attentionOnly: true });
		expect(filtered.map((r) => r.name)).toEqual([
			'payments-api',
			'backend',
			'cron-runner',
			'frontend'
		]);
	});

	it('treats several values in one facet as "any of these"', () => {
		const filtered = filterApplications(fleet(), {
			...emptyFacets(),
			health: new Set(['Degraded', 'Progressing'])
		});
		expect(filtered.map((r) => r.name)).toEqual(['payments-api', 'backend', 'cron-runner']);
	});

	it('ands separate facets together', () => {
		const filtered = filterApplications(fleet(), {
			...emptyFacets(),
			health: new Set(['Degraded']),
			sync: new Set(['Synced'])
		});
		expect(filtered.map((r) => r.name)).toEqual(['backend']);
	});

	it('matches names case-insensitively on a substring', () => {
		expect(
			filterApplications(fleet(), { ...emptyFacets(), name: 'END' }).map((r) => r.name)
		).toEqual(['backend', 'frontend']);
	});

	it('ignores surrounding whitespace in the name filter', () => {
		expect(filterApplications(fleet(), { ...emptyFacets(), name: '  gateway ' })).toHaveLength(1);
	});

	it('can filter everything out', () => {
		expect(filterApplications(fleet(), { ...emptyFacets(), name: 'nothing-matches-this' })).toEqual(
			[]
		);
	});
});

describe('toggleFacet', () => {
	it('adds a value that is not there and removes one that is', () => {
		const once = toggleFacet(new Set<string>(), 'Degraded');
		expect([...once]).toEqual(['Degraded']);

		expect([...toggleFacet(once, 'Degraded')]).toEqual([]);
	});

	it('returns a new set rather than mutating, so reactivity sees the change', () => {
		const original = new Set(['Degraded']);
		const next = toggleFacet(original, 'Healthy');

		expect(next).not.toBe(original);
		expect([...original]).toEqual(['Degraded']);
	});
});

describe('hasActiveFacets', () => {
	it('is false for untouched facets', () => {
		expect(hasActiveFacets(emptyFacets())).toBe(false);
	});

	it('is true once any one facet is set', () => {
		expect(hasActiveFacets({ ...emptyFacets(), attentionOnly: true })).toBe(true);
		expect(hasActiveFacets({ ...emptyFacets(), health: new Set(['Degraded']) })).toBe(true);
		expect(hasActiveFacets({ ...emptyFacets(), sync: new Set(['Synced']) })).toBe(true);
		expect(hasActiveFacets({ ...emptyFacets(), name: 'api' })).toBe(true);
	});

	it('ignores a name filter that is only whitespace', () => {
		expect(hasActiveFacets({ ...emptyFacets(), name: '   ' })).toBe(false);
	});
});

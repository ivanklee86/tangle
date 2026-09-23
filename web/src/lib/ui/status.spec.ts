import { describe, it, expect } from 'vitest';
import { statusAppearance, statusSeverity, needsAttention } from '$lib/ui/status';

describe('statusAppearance', () => {
	// The promise here is Argo CD's own vocabulary: a status means the same
	// thing in Tangle as in the Argo CD UI people came from. The colour is the
	// contract, not the icon — it's what a reader takes in first.
	it.each([
		['Healthy', 'green'],
		['Synced', 'green'],
		['OutOfSync', 'yellow'],
		['Progressing', 'blue'],
		['Degraded', 'red'],
		['Missing', 'red'],
		['Suspended', 'gray'],
		['Unknown', 'gray']
	])('renders %s as a %s badge', (status, color) => {
		expect(statusAppearance(status).color).toBe(color);
	});

	it('keeps OutOfSync away from the colour that means breakage', () => {
		// Drift is not breakage. If these ever collide, a fleet of
		// perfectly healthy but un-synced applications looks like an outage.
		expect(statusAppearance('OutOfSync').color).not.toBe(statusAppearance('Degraded').color);
	});

	it('falls back to gray for an unrecognised status', () => {
		// A status Argo CD adds later must not be reported as healthy.
		expect(statusAppearance('SomeNewStatus').color).toBe('gray');
		expect(statusAppearance('SomeNewStatus').needsAttention).toBe(false);
	});

	it('gives every status an icon, so colour is never the only signal', () => {
		for (const status of ['Healthy', 'OutOfSync', 'Progressing', 'Degraded', 'Unknown']) {
			expect(statusAppearance(status).icon).toBeDefined();
		}
	});
});

describe('statusSeverity', () => {
	it('ranks breakage above drift, and drift above healthy', () => {
		expect(statusSeverity('Degraded')).toBeGreaterThan(statusSeverity('Progressing'));
		expect(statusSeverity('Progressing')).toBeGreaterThan(statusSeverity('OutOfSync'));
		expect(statusSeverity('OutOfSync')).toBeGreaterThan(statusSeverity('Unknown'));
		expect(statusSeverity('Unknown')).toBeGreaterThan(statusSeverity('Healthy'));
	});

	it('ranks Degraded and Missing together', () => {
		expect(statusSeverity('Missing')).toBe(statusSeverity('Degraded'));
	});

	it('ranks Healthy and Synced together, at the bottom', () => {
		expect(statusSeverity('Synced')).toBe(statusSeverity('Healthy'));
		expect(statusSeverity('Healthy')).toBe(0);
	});
});

describe('needsAttention', () => {
	it('is false only when both statuses are clean', () => {
		expect(needsAttention('Healthy', 'Synced')).toBe(false);
	});

	it('is true when an application has drifted even though it is healthy', () => {
		// The case the Applications facet exists for: a running application
		// whose manifests no longer match what is deployed.
		expect(needsAttention('Healthy', 'OutOfSync')).toBe(true);
	});

	it('is true when health is bad even though sync is clean', () => {
		expect(needsAttention('Degraded', 'Synced')).toBe(true);
		expect(needsAttention('Missing', 'Synced')).toBe(true);
		expect(needsAttention('Progressing', 'Synced')).toBe(true);
	});

	it('leaves a suspended application alone', () => {
		// Suspended is somebody's deliberate pause, not a problem to surface.
		expect(needsAttention('Suspended', 'Synced')).toBe(false);
	});
});

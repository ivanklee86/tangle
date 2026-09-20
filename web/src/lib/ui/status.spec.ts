import { describe, it, expect } from 'vitest';
import { CheckCircleSolid, CloseCircleSolid, ExclamationCircleSolid } from 'flowbite-svelte-icons';
import { statusAppearance } from '$lib/ui/status';

describe('statusAppearance', () => {
	it('treats Healthy as healthy', () => {
		expect(statusAppearance('Healthy').icon).toBe(CheckCircleSolid);
	});

	it('treats Synced as healthy', () => {
		expect(statusAppearance('Synced').icon).toBe(CheckCircleSolid);
	});

	it('treats OutOfSync as unhealthy', () => {
		expect(statusAppearance('OutOfSync').icon).toBe(CloseCircleSolid);
	});

	it('treats Degraded as unhealthy', () => {
		expect(statusAppearance('Degraded').icon).toBe(CloseCircleSolid);
	});

	it('treats Missing as unhealthy', () => {
		expect(statusAppearance('Missing').icon).toBe(CloseCircleSolid);
	});

	it('treats Progressing as unknown', () => {
		expect(statusAppearance('Progressing').icon).toBe(ExclamationCircleSolid);
	});

	it('treats Suspended as unknown', () => {
		expect(statusAppearance('Suspended').icon).toBe(ExclamationCircleSolid);
	});

	it('treats Unknown as unknown', () => {
		expect(statusAppearance('Unknown').icon).toBe(ExclamationCircleSolid);
	});

	it('falls back to unknown for an unrecognized status', () => {
		expect(statusAppearance('SomeNewStatus').icon).toBe(ExclamationCircleSolid);
	});

	it('pairs each icon with a dark-mode-aware color class', () => {
		expect(statusAppearance('Healthy').class).toContain('dark:');
		expect(statusAppearance('OutOfSync').class).toContain('dark:');
		expect(statusAppearance('Unknown').class).toContain('dark:');
	});
});

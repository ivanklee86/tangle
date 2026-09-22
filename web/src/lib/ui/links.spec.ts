import { describe, it, expect, afterEach } from 'vitest';
import { absoluteUrl, baseUrl, setConfiguredDomain } from '$lib/ui/links';

afterEach(() => setConfiguredDomain(''));

describe('baseUrl', () => {
	it('prefers a configured domain', () => {
		setConfiguredDomain('https://tangle.corp');

		expect(baseUrl()).toBe('https://tangle.corp');
	});

	// The whole reason server-side configuration exists: someone reaching
	// Tangle through a port-forward sees localhost in their address bar, and a
	// link built from it is useless to anyone else.
	it('lets a configured domain win over the browser origin', () => {
		setConfiguredDomain('https://tangle.corp');

		expect(baseUrl()).not.toContain('localhost');
	});

	it('normalises a trailing slash, so joining a path never doubles up', () => {
		setConfiguredDomain('https://tangle.corp/');

		expect(absoluteUrl('/applications')).toBe('https://tangle.corp/applications');
	});

	it('ignores surrounding whitespace', () => {
		setConfiguredDomain('  https://tangle.corp  ');

		expect(baseUrl()).toBe('https://tangle.corp');
	});

	it('treats an empty configured domain as unset', () => {
		setConfiguredDomain('');

		// No window in the node test environment, so this is the "neither
		// source available" case.
		expect(baseUrl()).toBe('');
	});
});

describe('absoluteUrl', () => {
	it('joins a path onto the base', () => {
		setConfiguredDomain('https://tangle.corp');

		expect(absoluteUrl('/applications?labels=env%3Atest')).toBe(
			'https://tangle.corp/applications?labels=env%3Atest'
		);
	});

	it('keeps a configured sub-path, which is a legitimate deployment', () => {
		setConfiguredDomain('https://example.com/tangle');

		expect(absoluteUrl('/diffs')).toBe('https://example.com/tangle/diffs');
	});

	// A relative link is still usable in the address bar; a mangled absolute
	// one wouldn't be. Better to hand back what we were given.
	it('returns the path unchanged when there is no base at all', () => {
		expect(absoluteUrl('/applications')).toBe('/applications');
	});
});

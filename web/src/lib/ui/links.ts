/**
 * Where the links this UI offers to copy should point.
 *
 * Two sources, in order:
 *
 * 1. A `domain` configured on the server, when there is one. That exists for
 *    the case the browser can't know about — somebody reaching Tangle through
 *    a port-forward, whose address bar says `localhost:8081` and whose copied
 *    link is useless to a colleague.
 * 2. The browser's own origin otherwise, which is by construction the address
 *    that worked for the person copying.
 *
 * Set once from the root layout's load, before anything renders.
 */
let configuredDomain = '';

function setConfiguredDomain(domain: string): void {
	configuredDomain = domain.trim().replace(/\/$/, '');
}

/** The base every copyable link is built on, or '' if neither source is available. */
function baseUrl(): string {
	if (configuredDomain.length > 0) return configuredDomain;
	return typeof window === 'undefined' ? '' : window.location.origin;
}

/**
 * Turns a root-relative path into a full URL.
 *
 * Falls back to returning the path unchanged when there's no base to build on
 * — a relative link is still usable in the address bar, where a mangled
 * absolute one wouldn't be.
 */
function absoluteUrl(path: string): string {
	const base = baseUrl();
	return base.length > 0 ? `${base}${path}` : path;
}

export { absoluteUrl, baseUrl, setConfiguredDomain };

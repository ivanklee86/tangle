import { fetchConfig } from '$lib/backend/config';
import { setConfiguredDomain } from '$lib/ui/links';

export const prerender = true;
export const ssr = false;
export const trailingSlash = 'always';

// Awaited rather than streamed: the copyable links in the query editor need
// the base URL at first render, and showing a relative path that turns
// absolute a moment later is worse than the few milliseconds this same-origin
// request costs. fetchConfig never rejects — an older server without the
// endpoint just yields an empty domain, and the browser's origin takes over.
export const load = async ({ fetch }) => {
	const config = await fetchConfig(fetch);
	setConfiguredDomain(config.domain);

	return { config };
};

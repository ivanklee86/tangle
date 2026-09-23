import { PUBLIC_BASE_URL } from '$env/static/public';

interface TangleConfig {
	/** Public base URL of this Tangle, without a trailing slash. Empty when unset. */
	domain: string;
}

const EMPTY_CONFIG: TangleConfig = { domain: '' };

/**
 * Reads the settings only the running server knows.
 *
 * The frontend is one static bundle baked into the image and served by every
 * deployment, so a build-time variable can't carry anything
 * deployment-specific — this is the only route for a value like the public
 * domain.
 *
 * A failure is not an error worth surfacing: an older server has no such
 * endpoint, and everything this config affects has a sensible fallback. The
 * app renders either way.
 */
async function fetchConfig(fetchImpl: typeof fetch = fetch): Promise<TangleConfig> {
	try {
		const response = await fetchImpl(`${PUBLIC_BASE_URL}/api/config`);
		if (!response.ok) return EMPTY_CONFIG;

		const parsed = (await response.json()) as Partial<TangleConfig>;
		return { domain: typeof parsed.domain === 'string' ? parsed.domain : '' };
	} catch {
		return EMPTY_CONFIG;
	}
}

export { fetchConfig, EMPTY_CONFIG, type TangleConfig };

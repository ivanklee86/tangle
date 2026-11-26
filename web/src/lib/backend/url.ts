/**
 * Builds a query string, leaving out empty values. A list becomes one
 * parameter per non-empty item, in order (`targetRepo=a&targetRepo=b`).
 */
function buildQuery(params: Record<string, string | string[] | null | undefined>): string {
	const search = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (Array.isArray(value)) {
			for (const item of value) {
				if (item) search.append(key, item);
			}
		} else if (value) {
			search.set(key, value);
		}
	}
	const query = search.toString();
	return query ? `?${query}` : '';
}

export { buildQuery };

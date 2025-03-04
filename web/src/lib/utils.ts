import { type ArgoCDApplicationResults } from '$lib/data';

function filterOutZeroResults(results: ArgoCDApplicationResults[]): ArgoCDApplicationResults[] {
	return results.filter((result) => result.applications.length > 0);
}

export { filterOutZeroResults };

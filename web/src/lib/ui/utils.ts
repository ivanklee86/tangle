import { type ArgoCDApplicationResults } from '#lib/backend/data.ts';

function filterOutZeroResults(results: ArgoCDApplicationResults[]): ArgoCDApplicationResults[] {
	return results.filter((result) => result.applications.length > 0);
}

export { filterOutZeroResults };

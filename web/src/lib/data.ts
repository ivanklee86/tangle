import { writable, derived } from 'svelte/store';

interface ApplicationLinks {
	name: string;
	url: string;
}

interface ArgoCDApplicationResults {
	name: string;
	link: string;
	applications: ApplicationLinks[];
}

interface ApplicationsResponse {
	results: ArgoCDApplicationResults[];
}

export const apiData = writable<ApplicationsResponse>({ results: [] });

export const title = derived(apiData, ($apiData) => {
	return $apiData.results.map((result) => result.name).join(', ');
});

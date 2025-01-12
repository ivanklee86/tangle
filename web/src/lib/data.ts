import { writable } from 'svelte/store';

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

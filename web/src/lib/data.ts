import { writable } from 'svelte/store';

interface TangleError {
	error: string;
}

interface ApplicationLinks {
	name: string;
	url: string;
	health: string;
	syncStatus: string;
}

interface ArgoCDApplicationResults {
	name: string;
	link: string;
	applications: ApplicationLinks[];
}

interface ApplicationsResponse {
	results: ArgoCDApplicationResults[];
}

interface ApplicationResponseStore {
	response: ApplicationsResponse;
	errorResponse: TangleError | null;
	error: boolean;
}

interface ApplicationDiffResponse {
	liveManifests: string;
	targetManifests: string;
	diff: string;
}

interface ApplicationDiff {
	response: ApplicationDiffResponse;
	errorResponse: TangleError | null;
	error: boolean;
}

export const apiData = writable<ApplicationResponseStore>({
	response: { results: [] },
	errorResponse: { error: '' },
	error: false
});

export const diffData = writable<ApplicationDiff>({
	response: { liveManifests: '', targetManifests: '', diff: '' },
	errorResponse: { error: '' },
	error: false
});

export type { ApplicationsResponse, ApplicationResponseStore, ApplicationDiff, TangleError };

import { writable } from 'svelte/store';

interface TangleError {
	error: string;
}

interface ApplicationLinks {
	name: string;
	url: string;
	health: string;
	syncStatus: string;
	liveRef: string;
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
	loaded: boolean;
}

interface ApplicationDiffResponse {
	liveManifests: string;
	targetManifests: string;
	diffs: string;
	manifestGenerationError: string;
}

interface ApplicationDiff {
	response: ApplicationDiffResponse;
	errorResponse: TangleError | null;
	requestDetails: {
		argoCD: string;
		applicationName: string;
	};
	error: boolean;
	loaded: boolean;
}

interface ApplicationsDiffsData {
	[argoCDName: string]: {
		[applicationName: string]: ApplicationDiff;
	};
}

export const apiData = writable<ApplicationResponseStore>({
	response: { results: [] },
	errorResponse: { error: '' },
	error: false,
	loaded: false
});

export const diffData = writable<ApplicationDiff>({
	response: { liveManifests: '', targetManifests: '', diffs: '', manifestGenerationError: '' },
	errorResponse: { error: '' },
	requestDetails: { argoCD: '', applicationName: '' },
	error: false,
	loaded: false
});

export type {
	ApplicationsResponse,
	ApplicationResponseStore,
	ApplicationDiff,
	ApplicationsDiffsData,
	ApplicationLinks,
	ApplicationDiffResponse,
	ArgoCDApplicationResults,
	TangleError
};

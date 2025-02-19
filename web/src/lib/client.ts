import {
	type ApplicationsResponse,
	type ApplicationResponseStore,
	type ApplicationDiff,
	type ApplicationDiffResponse,
	type TangleError
} from '$lib/data';
import { PUBLIC_BASE_URL } from '$env/static/public';

const PATH_APPLICATIONS: string = '/api/applications';

class TangleAPIClient {
	baseUrl: string;

	constructor() {
		this.baseUrl = PUBLIC_BASE_URL;
	}

	async getApplications(labels: string | null): Promise<ApplicationResponseStore> {
		const url = labels
			? `${this.baseUrl}${PATH_APPLICATIONS}?labels=${labels}`
			: `${this.baseUrl}${PATH_APPLICATIONS}`;

		let store: ApplicationResponseStore;

		try {
			const response = await fetch(url);
			const data = await response.json();

			let store: ApplicationResponseStore;
			if (response.status !== 200) {
				store = {
					response: { results: [] },
					errorResponse: data as TangleError,
					error: true,
					loaded: true
				};
			} else {
				store = {
					response: data as ApplicationsResponse,
					errorResponse: { error: '' },
					error: false,
					loaded: true
				};
			}
			return store;
		} catch (error) {
			console.error('Failed to fetch applications:', error);
			store = {
				response: { results: [] },
				errorResponse: {
					error: error as string
				},
				error: true,
				loaded: true
			};
			return store;
		}
	}

	async getApplicationDiff(
		argoCD: string,
		applicationName: string,
		liveRef: string,
		targetRef: string
	): Promise<ApplicationDiff> {
		const url = `${this.baseUrl}/api/argocd/${argoCD}/applications/${applicationName}/diffs`;
		const body = {
			liveRef: liveRef,
			targetRef: targetRef
		};

		try {
			const response = await fetch(url, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify(body)
			});
			const data = await response.json();

			let appDiffResp: ApplicationDiff;
			if (response.status !== 200) {
				appDiffResp = {
					response: {
						liveManifests: '',
						targetManifests: '',
						diffs: '',
						manifestGenerationError: ''
					},
					requestDetails: {
						argoCD: argoCD,
						applicationName: applicationName
					},
					errorResponse: data as TangleError,
					error: true,
					loaded: true
				};
			} else {
				appDiffResp = {
					response: data as ApplicationDiffResponse,
					errorResponse: { error: '' },
					requestDetails: {
						argoCD: argoCD,
						applicationName: applicationName
					},
					error: false,
					loaded: true
				};
			}

			return appDiffResp;
		} catch (error) {
			console.error('Failed to fetch applications:', error);

			const errResp = {
				response: {
					liveManifests: '',
					targetManifests: '',
					diffs: '',
					manifestGenerationError: ''
				},
				errorResponse: {
					error: error as string
				},
				requestDetails: {
					argoCD: argoCD,
					applicationName: applicationName
				},
				error: true,
				loaded: true
			};

			return errResp;
		}
	}
}

export default TangleAPIClient;

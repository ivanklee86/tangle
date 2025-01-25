import {
	type ApplicationsResponse,
	type ApplicationResponseStore,
	type ApplicationDiff,
	type ApplicationDiffResponse,
	type TangleError
} from '$lib/data';
import { apiData } from '$lib/data';
import { PUBLIC_BASE_URL } from '$env/static/public';
import { error } from '@sveltejs/kit';

const PATH_APPLICATIONS: string = '/api/applications';

class TangleAPIClient {
	baseUrl: string;

	constructor() {
		this.baseUrl = PUBLIC_BASE_URL;
	}

	async getApplications(labels: string | null): Promise<void> {
		const url = labels
			? `${this.baseUrl}${PATH_APPLICATIONS}?labels=${labels}`
			: `${this.baseUrl}${PATH_APPLICATIONS}`;

		let store: ApplicationResponseStore;

		try {
			const response = await fetch(url);
			const data = await response.json();

			if (response.status !== 200) {
				store = {
					response: { results: [] },
					errorResponse: data as TangleError,
					error: true
				};
			} else {
				store = {
					response: data as ApplicationsResponse,
					errorResponse: { error: '' },
					error: false
				};
			}

			apiData.set(store);
		} catch (error) {
			console.error('Failed to fetch applications:', error);
			apiData.set({
				response: { results: [] },
				errorResponse: {
					error: error as string
				},
				error: true
			});
		}
	}

	async getApplicationDiff(argoCD: string, applicationName: string, liveRef: string, targetRef: string): Promise<ApplicationDiff> {
		const url =`${this.baseUrl}/api/argocd/${argoCD}/applications/${applicationName}/diffs`;
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
			})
			const data = await response.json();
			
			let appDiffResp: ApplicationDiff;
			if (response.status !== 200) {
				appDiffResp = {
					response: {
						liveManifests: '',
						targetManifests: '',
						diff: ''
					},
					errorResponse: data as TangleError,
					error: true
				};
			} else {
				appDiffResp = {
					response: data as ApplicationDiffResponse,
					errorResponse: { error: '' },
					error: false
				};
			};

			return appDiffResp
		} catch (error) {
			console.error('Failed to fetch applications:', error);

			const errResp = {
				response: {
					liveManifests: '',
					targetManifests: '',
					diff: ''
				},
				errorResponse: {
					error: error as string
				},
				error: true
			}

			return errResp;
		}
	}
}

export default TangleAPIClient;

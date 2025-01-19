import {
	type ApplicationsResponse,
	type ApplicationResponseStore,
	type TangleError
} from '$lib/data';
import { apiData } from '$lib/data';
import { PUBLIC_BASE_URL } from '$env/static/public';

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
}

export default TangleAPIClient;

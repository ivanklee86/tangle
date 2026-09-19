import {
	type ApplicationsResponse,
	type ApplicationResponseStore,
	type ApplicationDiff,
	type ApplicationDiffResponse
} from '$lib/backend/data';
import { PUBLIC_BASE_URL } from '$env/static/public';
import { fetchEnvelope } from '$lib/backend/http';
import { buildQuery } from '$lib/backend/url';

const PATH_APPLICATIONS: string = '/api/applications';

const EMPTY_APPLICATIONS_RESPONSE: ApplicationsResponse = { results: [] };
const EMPTY_APPLICATION_DIFF_RESPONSE: ApplicationDiffResponse = {
	liveManifests: '',
	targetManifests: '',
	diffs: '',
	manifestGenerationError: ''
};

class TangleAPIClient {
	baseUrl: string;
	private fetchImpl: typeof fetch;

	constructor(fetchImpl: typeof fetch = fetch) {
		this.baseUrl = PUBLIC_BASE_URL;
		this.fetchImpl = fetchImpl;
	}

	async getApplications(
		labels: string | null,
		excludeLabels: string | null
	): Promise<ApplicationResponseStore> {
		const url = `${this.baseUrl}${PATH_APPLICATIONS}${buildQuery({ labels, excludeLabels })}`;

		return fetchEnvelope<ApplicationsResponse>(
			url,
			EMPTY_APPLICATIONS_RESPONSE,
			undefined,
			this.fetchImpl
		);
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

		const envelope = await fetchEnvelope<ApplicationDiffResponse>(
			url,
			EMPTY_APPLICATION_DIFF_RESPONSE,
			{
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify(body)
			},
			this.fetchImpl
		);

		return { ...envelope, requestDetails: { argoCD, applicationName } };
	}
}

export default TangleAPIClient;

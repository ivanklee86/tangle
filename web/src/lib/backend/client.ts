import {
	type ApplicationsResponse,
	type ApplicationResponseStore,
	type ApplicationDiff,
	type ApplicationDiffResponse
} from '#lib/backend/data.ts';
import { PUBLIC_BASE_URL } from '$app/env/public';
import { fetchEnvelope } from '#lib/backend/http.ts';
import { buildQuery } from '#lib/backend/url.ts';

/** Which sources of an application get the target ref (ADR 0032). */
interface DiffOptions {
	targetRepos?: string[];
	inferTargetRepo?: boolean;
}

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
		targetRef: string,
		options: DiffOptions = {}
	): Promise<ApplicationDiff> {
		const url = `${this.baseUrl}/api/argocd/${argoCD}/applications/${applicationName}/diffs`;
		// The new fields only go out when set, so the request a single-source
		// diff sends is byte-for-byte what it always was.
		const body: Record<string, unknown> = { liveRef, targetRef };
		if (options.targetRepos && options.targetRepos.length > 0)
			body.targetRepos = options.targetRepos;
		if (options.inferTargetRepo) body.inferTargetRepo = true;

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
export type { DiffOptions };

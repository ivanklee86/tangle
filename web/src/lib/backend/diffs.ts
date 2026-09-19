import {
	type ArgoCDApplicationResults,
	type ApplicationDiff,
	type ApplicationsDiffsData
} from '$lib/backend/data';
import type TangleAPIClient from '$lib/backend/client';

interface DiffRequest {
	argoCDName: string;
	applicationName: string;
	liveRef: string;
	targetRef: string;
}

function buildDiffRequests(
	results: ArgoCDApplicationResults[],
	targetRef: string | null
): DiffRequest[] {
	return results.flatMap((argoCD) =>
		argoCD.applications.map((application) => ({
			argoCDName: argoCD.name,
			applicationName: application.name,
			liveRef: application.liveRef,
			targetRef: targetRef ? targetRef : application.liveRef
		}))
	);
}

function buildDiffMap(diffs: ApplicationDiff[]): ApplicationsDiffsData {
	const map: ApplicationsDiffsData = {};
	diffs.forEach((diff) => {
		const { argoCD, applicationName } = diff.requestDetails;
		if (!map[argoCD]) {
			map[argoCD] = {};
		}
		map[argoCD][applicationName] = diff;
	});
	return map;
}

interface FetchDiffsCallbacks {
	onTotal?: (total: number) => void;
	onProgress?: () => void;
}

async function fetchDiffs(
	client: TangleAPIClient,
	results: ArgoCDApplicationResults[],
	targetRef: string | null,
	callbacks: FetchDiffsCallbacks = {}
): Promise<ApplicationsDiffsData> {
	const requests = buildDiffRequests(results, targetRef);
	callbacks.onTotal?.(requests.length);

	const diffs = await Promise.all(
		requests.map((request) =>
			client
				.getApplicationDiff(
					request.argoCDName,
					request.applicationName,
					request.liveRef,
					request.targetRef
				)
				.then((result) => {
					callbacks.onProgress?.();
					return result;
				})
		)
	);

	return buildDiffMap(diffs);
}

export { buildDiffRequests, buildDiffMap, fetchDiffs, type DiffRequest, type FetchDiffsCallbacks };

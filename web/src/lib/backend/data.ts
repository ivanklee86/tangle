interface TangleError {
	error: string;
}

/** git, helm or oci: how ArgoCD fetches a source. */
type SourceKind = 'git' | 'helm' | 'oci';

/** One entry of an application's sources, as /api/applications lists it. */
interface ApplicationSource {
	/** 1-based, as ArgoCD counts source positions. */
	position: number;
	kind: SourceKind;
	repoURL: string;
	path?: string;
	chart?: string;
	ref?: string;
	targetRevision: string;
	/** Positions of the sources that read files through this ref (`$values/...`). */
	usedBy?: number[];
}

interface ApplicationLinks {
	name: string;
	url: string;
	health: string;
	syncStatus: string;
	/** spec.source.targetRevision; empty for an application with spec.sources. */
	liveRef: string;
	/** Absent from a server that predates multi-source support. */
	sources?: ApplicationSource[];
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

/** ok, notAffected, needsTargetRepo or error (ADR 0032). */
type DiffStatus = 'ok' | 'notAffected' | 'needsTargetRepo' | 'error';

/** A source with the revision each side of a diff used. */
interface DiffSource extends Omit<ApplicationSource, 'targetRevision'> {
	liveRevision: string;
	targetRevision: string;
	/** True when this source got the target ref. */
	moved: boolean;
}

interface ApplicationDiffResponse {
	liveManifests: string;
	targetManifests: string;
	diffs: string;
	manifestGenerationError: string;
	// The rest is absent from a server that predates multi-source support.
	status?: DiffStatus;
	sources?: DiffSource[];
	/** The normalized repos the target ref went to. */
	targetRepos?: string[];
	inferredTargetRepo?: boolean;
	/** For needsTargetRepo: the app's git repos, normalized, in source order. */
	candidateRepos?: string[];
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

function emptyApplicationResponseStore(): ApplicationResponseStore {
	return {
		response: { results: [] },
		errorResponse: { error: '' },
		error: false,
		loaded: false
	};
}

export { emptyApplicationResponseStore };

export type {
	ApplicationSource,
	ApplicationsResponse,
	DiffSource,
	DiffStatus,
	SourceKind,
	ApplicationResponseStore,
	ApplicationDiff,
	ApplicationsDiffsData,
	ApplicationLinks,
	ApplicationDiffResponse,
	ArgoCDApplicationResults,
	TangleError
};

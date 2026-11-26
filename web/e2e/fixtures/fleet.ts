import type { Page, Route } from '@playwright/test';

// A realistic fleet for the screenshot tests and the documentation
// screenshots: two ArgoCDs, applications named the way a platform team names
// them, every health and sync state, and single- and multi-source
// applications. Unlike the integration fixtures (test-1, env:test), these are
// meant to be looked at.
//
// The diff responder answers each application the way tangle-server would
// (ADR 0032) for the request's target ref, target repos and inference, so a
// screenshot of any query shows what the real UI would. Everything is fixed,
// including the diff headers the real server fills with temp paths and
// times, so screenshots don't vary between runs.

type Kind = 'git' | 'helm' | 'oci';

interface FleetSource {
	kind: Kind;
	repoURL: string;
	path?: string;
	chart?: string;
	ref?: string;
	targetRevision: string;
	usedBy?: number[];
}

/** What rendering at the target ref finds, for an application that is rendered. */
type Rendered = { diff: string } | { unchanged: true } | { error: string };

interface FleetApp {
	name: string;
	health: string;
	syncStatus: string;
	sources: FleetSource[];
	rendered: Rendered;
}

const GITHUB = 'https://github.com/acme';

function diff(resource: string, lines: string[]): string {
	return [
		'--- live.yaml\t2026-11-26 09:00:00.000000000 +0000',
		'+++ target.yaml\t2026-11-26 09:00:00.000000000 +0000',
		`@@ -1,${lines.length} +1,${lines.length} @@`,
		` ${resource}`,
		...lines
	].join('\n');
}

const CHECKOUT_DIFF = diff('apiVersion: apps/v1', [
	' kind: Deployment',
	' metadata:',
	'   name: checkout-api',
	' spec:',
	'-  replicas: 3',
	'+  replicas: 4',
	'   template:',
	'     spec:',
	'       containers:',
	'         - name: checkout-api',
	'-          image: ghcr.io/acme/checkout-api:1.42.0',
	'+          image: ghcr.io/acme/checkout-api:1.43.0',
	'           resources:',
	'             limits:',
	'-              memory: 512Mi',
	'+              memory: 768Mi'
]);

const PAYMENTS_DIFF = diff('apiVersion: v1', [
	' kind: ConfigMap',
	' metadata:',
	'   name: payments-worker',
	' data:',
	'-  QUEUE_CONCURRENCY: "8"',
	'+  QUEUE_CONCURRENCY: "16"',
	'   RETRY_BACKOFF: 30s'
]);

const PODINFO_DIFF = diff('apiVersion: autoscaling/v2', [
	' kind: HorizontalPodAutoscaler',
	' metadata:',
	'   name: podinfo',
	' spec:',
	'-  maxReplicas: 4',
	'+  maxReplicas: 8',
	'   minReplicas: 2'
]);

const FLEET: { instance: string; apps: FleetApp[] }[] = [
	{
		instance: 'prod',
		apps: [
			{
				name: 'checkout-api',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'git',
						repoURL: `${GITHUB}/checkout-api.git`,
						path: 'deploy/prod',
						targetRevision: 'main'
					}
				],
				rendered: { diff: CHECKOUT_DIFF }
			},
			{
				name: 'payments-worker',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'helm',
						repoURL: 'https://charts.acme.dev/stable',
						chart: 'worker',
						targetRevision: '2.4.1'
					},
					{
						kind: 'git',
						repoURL: `${GITHUB}/platform-values.git`,
						ref: 'values',
						targetRevision: 'main',
						usedBy: [1]
					}
				],
				rendered: { diff: PAYMENTS_DIFF }
			},
			{
				name: 'ledger',
				health: 'Degraded',
				syncStatus: 'OutOfSync',
				sources: [
					{ kind: 'git', repoURL: `${GITHUB}/ledger.git`, path: 'chart', targetRevision: 'main' }
				],
				rendered: {
					error:
						'rpc error: code = Unknown desc = failed to execute helm template command: YAML parse error on ledger/templates/deployment.yaml: error converting YAML to JSON: yaml: line 12: did not find expected key'
				}
			},
			{
				name: 'search-indexer',
				health: 'Progressing',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'git',
						repoURL: `${GITHUB}/search.git`,
						path: 'deploy/indexer',
						targetRevision: 'main'
					}
				],
				rendered: { unchanged: true }
			},
			{
				name: 'notifications',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'helm',
						repoURL: 'https://charts.acme.dev/stable',
						chart: 'notifications',
						targetRevision: '4.1.0'
					}
				],
				rendered: { unchanged: true }
			}
		]
	},
	{
		instance: 'staging',
		apps: [
			{
				name: 'checkout-api',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'git',
						repoURL: `${GITHUB}/checkout-api.git`,
						path: 'deploy/staging',
						targetRevision: 'main'
					}
				],
				rendered: { diff: CHECKOUT_DIFF }
			},
			{
				name: 'podinfo-stack',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'git',
						repoURL: 'https://github.com/stefanprodan/podinfo.git',
						path: 'kustomize',
						targetRevision: '6.17.0'
					},
					{
						kind: 'git',
						repoURL: `${GITHUB}/platform-values.git`,
						path: 'overlays/staging',
						targetRevision: 'main'
					},
					{
						kind: 'git',
						repoURL: `${GITHUB}/legacy-config.git`,
						path: 'podinfo',
						targetRevision: '3f9c2a17be04d1c8e6a5f0b9d27c4e81a3b6f5d0'
					}
				],
				rendered: { diff: PODINFO_DIFF }
			},
			{
				name: 'feature-flags',
				health: 'Healthy',
				syncStatus: 'Synced',
				sources: [
					{
						kind: 'git',
						repoURL: `${GITHUB}/feature-flags.git`,
						path: 'deploy',
						targetRevision: 'v2.8.1'
					}
				],
				rendered: { unchanged: true }
			},
			{
				name: 'billing-cron',
				health: 'Missing',
				syncStatus: 'OutOfSync',
				sources: [
					{ kind: 'git', repoURL: `${GITHUB}/billing.git`, path: 'cron', targetRevision: 'main' }
				],
				rendered: { unchanged: true }
			}
		]
	}
];

/** host/path, the way the server compares repos. */
function normalize(repo: string): string {
	return repo
		.trim()
		.replace(/^[a-z+]+:\/\//i, '')
		.replace(/^[^@/]+@/, '')
		.replace(/:(?!\d+\/)/, '/')
		.replace(/\/+$/, '')
		.replace(/\.git$/, '')
		.replace(/^([^/]+)/, (host) => host.toLowerCase().replace(/:\d+$/, ''));
}

function applicationsResponse() {
	return {
		results: FLEET.map(({ instance, apps }) => ({
			name: instance,
			link: `https://argocd-${instance}.acme.dev/applications`,
			applications: apps.map((app) => ({
				name: app.name,
				url: `https://argocd-${instance}.acme.dev/applications/argocd/${app.name}`,
				health: app.health,
				syncStatus: app.syncStatus,
				liveRef: app.sources.length === 1 ? app.sources[0].targetRevision : '',
				sources: app.sources.map((source, index) => ({ position: index + 1, ...source }))
			}))
		}))
	};
}

interface DiffRequest {
	liveRef: string;
	targetRef: string;
	targetRepos?: string[];
	inferTargetRepo?: boolean;
}

const EMPTY = { liveManifests: '', targetManifests: '', diffs: '', manifestGenerationError: '' };

/** The server's answer for one application (ADR 0032's planning rules). */
function diffResponse(app: FleetApp, request: DiffRequest) {
	const gitRepos = [
		...new Set(app.sources.filter((s) => s.kind === 'git').map((s) => normalize(s.repoURL)))
	];
	let wanted = (request.targetRepos ?? []).map(normalize);
	const multi = app.sources.length > 1;

	if (multi && wanted.length === 0) {
		if (!request.inferTargetRepo || gitRepos.length !== 1) {
			return {
				...EMPTY,
				status: 'needsTargetRepo',
				manifestGenerationError: `needs target repo: ${app.name} has git sources in ${gitRepos.length} repos`,
				candidateRepos: gitRepos,
				sources: app.sources.map((source, index) => ({
					position: index + 1,
					...source,
					liveRevision: source.targetRevision,
					moved: false
				}))
			};
		}
		wanted = gitRepos;
	}

	const moves = (source: FleetSource) =>
		wanted.length === 0
			? !multi
			: source.kind === 'git' && wanted.includes(normalize(source.repoURL));
	const sources = app.sources.map((source, index) => ({
		position: index + 1,
		...source,
		liveRevision: source.targetRevision,
		targetRevision: moves(source) ? request.targetRef : source.targetRevision,
		moved: moves(source)
	}));

	if (!sources.some((source) => source.moved)) {
		return { ...EMPTY, status: 'notAffected', sources };
	}

	const rendered = app.rendered;
	if ('error' in rendered) {
		return { ...EMPTY, status: 'error', manifestGenerationError: rendered.error };
	}
	const diffs = 'diff' in rendered ? rendered.diff : '';
	return {
		...EMPTY,
		liveManifests: `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${app.name}\n`,
		targetManifests: `apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${app.name}\n`,
		diffs,
		status: 'ok',
		sources,
		targetRepos: wanted,
		inferredTargetRepo: multi && (request.targetRepos ?? []).length === 0
	};
}

/** Serves the fleet for every Tangle API call the page makes. */
async function mockFleetAPI(page: Page): Promise<void> {
	await page.route('**/api/config', (route) =>
		route.fulfill({ json: { domain: 'https://tangle.acme.dev' } })
	);
	await page.route('**/api/applications*', (route) =>
		route.fulfill({ json: applicationsResponse() })
	);
	await page.route('**/api/argocd/*/applications/*/diffs', (route: Route) => {
		const [, instance, name] =
			/\/api\/argocd\/([^/]+)\/applications\/([^/]+)\/diffs/.exec(route.request().url()) ?? [];
		const app = FLEET.find((entry) => entry.instance === instance)?.apps.find(
			(entry) => entry.name === name
		);
		if (!app)
			return route.fulfill({ status: 404, json: { error: `no fixture for ${instance}/${name}` } });
		return route.fulfill({
			json: diffResponse(app, route.request().postDataJSON() as DiffRequest)
		});
	});
}

export { mockFleetAPI };

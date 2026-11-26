/**
 * A repo URL short enough to show inline: `https://github.com/org/values.git`
 * reads `org/values`. The host is kept only when the path alone would be one
 * bare word (a Helm repo such as `https://stefanprodan.github.io/podinfo`), so
 * the result still says where it is. Show the full URL alongside, on hover.
 *
 * Accepts the server's normalized `host/path` form too.
 */
function shortRepo(repo: string): string {
	let rest = repo.trim();
	if (rest.length === 0) return '';

	rest = rest.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
	// scp-style `git@host:path`
	const scp = /^(?:[^@/]+@)?([^/:]+):(?!\d+\/)(.+)$/.exec(rest);
	if (scp) rest = `${scp[1]}/${scp[2]}`;
	rest = rest.replace(/^[^@/]+@/, '');

	const [host, ...pathParts] = rest.split('/');
	const path = pathParts
		.join('/')
		.replace(/\/+$/, '')
		.replace(/\.git$/, '');
	const hostname = host.replace(/:\d+$/, '');

	if (path.length === 0) return hostname;
	return path.includes('/') ? path : `${hostname}/${path}`;
}

export { shortRepo };

/**
 * A revision short enough to show inline: a full 40-character commit SHA reads
 * as its first 7 characters, the way git abbreviates it. Branches, tags and
 * chart versions are left alone. Show the full value alongside, on hover.
 */
function shortRevision(revision: string): string {
	return /^[0-9a-f]{40}$/i.test(revision) ? revision.slice(0, 7) : revision;
}

export { shortRevision };

/**
 * A git repo URL reduced to `host/path`, the way the server matches target
 * repos: https, ssh and scp-style spellings, `.git`, a trailing slash,
 * credentials, a port and the host's case all compare equal. Mirrors
 * NormalizeRepoURL in internal/argocd/sources.go, so counts here agree with
 * what the server will match.
 */
function normalizeRepo(raw: string): string {
	const trimmed = raw.trim();
	if (trimmed.length === 0) return '';

	let host: string;
	let path: string;
	if (trimmed.includes('://')) {
		try {
			const url = new URL(trimmed);
			if (url.hostname.length === 0) return trimmed.toLowerCase();
			host = url.hostname;
			path = url.pathname;
		} catch {
			return trimmed.toLowerCase();
		}
	} else {
		const scp = /^(?:[^@/]+@)?([^/:]+):(.+)$/.exec(trimmed);
		if (scp) {
			host = scp[1];
			path = scp[2];
			// A bare `host:8443/path` means a port; scp syntax always has `user@`.
			if (!trimmed.includes('@')) path = path.replace(/^\d+\//, '');
		} else {
			// A bare `host/path`, as people tend to type into a form.
			const slash = trimmed.indexOf('/');
			host = slash === -1 ? trimmed : trimmed.slice(0, slash);
			path = slash === -1 ? '' : trimmed.slice(slash + 1);
		}
	}

	path = path
		.replace(/^\/+|\/+$/g, '')
		.replace(/\.git$/, '')
		.replace(/^\/+|\/+$/g, '');
	return `${host.toLowerCase()}/${path}`;
}

interface RepoUsage {
	/** Normalized, for matching. */
	repo: string;
	/** The URL as an application spells it, for display and for adding. */
	url: string;
	/** How many applications have at least one git source in it. */
	apps: number;
}

/**
 * Which git repos a set of applications use, and how many applications use
 * each, most used first. Each application is given as its sources.
 */
function repoUsage(applications: { kind: string; repoURL: string }[][]): RepoUsage[] {
	const usage = new Map<string, RepoUsage>();
	for (const sources of applications) {
		const seen = new Set<string>();
		for (const source of sources) {
			if (source.kind !== 'git') continue;
			const repo = normalizeRepo(source.repoURL);
			if (repo.length === 0 || seen.has(repo)) continue;
			seen.add(repo);
			const entry = usage.get(repo) ?? { repo, url: source.repoURL, apps: 0 };
			entry.apps += 1;
			usage.set(repo, entry);
		}
	}
	return [...usage.values()].sort((a, b) => b.apps - a.apps || a.repo.localeCompare(b.repo));
}

/** "used by source 1", "used by sources 1 and 3", or '' for none. */
function usedByLabel(usedBy: number[] | undefined): string {
	if (!usedBy || usedBy.length === 0) return '';
	if (usedBy.length === 1) return `used by source ${usedBy[0]}`;
	return `used by sources ${usedBy.slice(0, -1).join(', ')} and ${usedBy[usedBy.length - 1]}`;
}

export { normalizeRepo, repoUsage, usedByLabel, type RepoUsage };

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

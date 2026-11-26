import { describe, expect, it } from 'vitest';
import { normalizeRepo, repoUsage, shortRepo, shortRevision, usedByLabel } from '$lib/ui/repos';

// Test cases:
//   - an https, ssh or scp-style git URL shortens to org/repo, without .git or
//     a trailing slash
//   - the server's normalized host/path form shortens the same way
//   - a one-segment path keeps its host, so a Helm repo still says where it is
//   - a deeper path is kept whole (GitLab subgroups)
//   - blank input is blank
describe('shortRepo', () => {
	it.each([
		'https://github.com/org/values.git',
		'https://github.com/org/values',
		'https://github.com/org/values/',
		'ssh://git@github.com:22/org/values.git',
		'git@github.com:org/values.git',
		'github.com/org/values'
	])('shortens %s to org/values', (repo) => {
		expect(shortRepo(repo)).toBe('org/values');
	});

	it('keeps the host when the path is one word', () => {
		expect(shortRepo('https://stefanprodan.github.io/podinfo')).toBe(
			'stefanprodan.github.io/podinfo'
		);
	});

	it('keeps a deeper path whole', () => {
		expect(shortRepo('https://gitlab.example.com/platform/team/config.git')).toBe(
			'platform/team/config'
		);
	});

	it('is blank for blank input', () => {
		expect(shortRepo('  ')).toBe('');
	});
});

// Test cases:
//   - a full SHA reads as its first 7 characters
//   - branches, tags, chart versions and short SHAs are left alone
describe('shortRevision', () => {
	it('abbreviates a full commit SHA', () => {
		expect(shortRevision('309da004053082d138071fb5494d0b510bc23473')).toBe('309da00');
	});

	it.each(['main', 'fixture-pinned-v1', '6.17.0', '309da00', 'feature/x'])(
		'leaves %s alone',
		(revision) => {
			expect(shortRevision(revision)).toBe(revision);
		}
	);
});

// Test cases (normalizeRepo mirrors the server's NormalizeRepoURL, so these
// are the spellings internal/argocd/sources_test.go checks):
//   - every spelling of one repo normalizes the same
//   - a bare host:port is a port, scp syntax's numeric segment is a path
//   - the path keeps its case, and blank is blank
describe('normalizeRepo', () => {
	it.each([
		'https://github.com/org/repo',
		'https://github.com/org/repo.git',
		'https://github.com/org/repo/',
		'https://github.com/org/repo.git/',
		'https://GitHub.com/org/repo',
		'https://user:token@github.com/org/repo.git',
		'http://github.com/org/repo',
		'ssh://git@github.com/org/repo.git',
		'ssh://git@github.com:22/org/repo.git',
		'git@github.com:org/repo.git',
		'git@github.com:org/repo',
		'github.com/org/repo',
		'github.com:443/org/repo',
		'  https://github.com/org/repo.git  '
	])('normalizes %s', (spelling) => {
		expect(normalizeRepo(spelling)).toBe('github.com/org/repo');
	});

	it('reads a bare host:port as a port and scp syntax as a path', () => {
		expect(normalizeRepo('git.example.com:8443/org/repo')).toBe(
			normalizeRepo('https://git.example.com:8443/org/repo.git')
		);
		expect(normalizeRepo('git@git.example.com:2024/repo.git')).toBe('git.example.com/2024/repo');
	});

	it('keeps the path case, and blank is blank', () => {
		expect(normalizeRepo('https://github.com/Org/Repo.git')).toBe('github.com/Org/Repo');
		expect(normalizeRepo('  ')).toBe('');
	});
});

// Test cases:
//   - counts applications per git repo, once per application however many
//     of its sources are in it
//   - Helm and OCI sources don't count
//   - spellings of one repo count together, keeping the first URL seen
//   - most used first, then by name
describe('repoUsage', () => {
	const git = (repoURL: string) => ({ kind: 'git', repoURL });

	it('counts applications per repo', () => {
		expect(
			repoUsage([
				[git('https://github.com/org/values.git'), git('https://github.com/org/values')],
				[
					git('git@github.com:org/values.git'),
					{ kind: 'helm', repoURL: 'https://charts.example.com' }
				],
				[git('https://github.com/org/apps.git')]
			])
		).toEqual([
			{ repo: 'github.com/org/values', url: 'https://github.com/org/values.git', apps: 2 },
			{ repo: 'github.com/org/apps', url: 'https://github.com/org/apps.git', apps: 1 }
		]);
	});
});

describe('usedByLabel', () => {
	it.each([
		[undefined, ''],
		[[], ''],
		[[1], 'used by source 1'],
		[[1, 3], 'used by sources 1 and 3'],
		[[1, 2, 4], 'used by sources 1, 2 and 4']
	])('labels %j as "%s"', (usedBy, label) => {
		expect(usedByLabel(usedBy)).toBe(label);
	});
});

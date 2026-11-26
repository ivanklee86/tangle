import { describe, expect, it } from 'vitest';
import { shortRepo } from '$lib/ui/repos';

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

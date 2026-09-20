import { defineConfig } from '@playwright/test';

// No webServer block, unlike playwright.config.ts — this suite targets a
// real tangle-server (built into the Docker image, `task services:cicd`
// brings it up on :8081) backed by a real ArgoCD, not a `vite preview` of
// the local checkout. The caller is responsible for that stack already
// being up before running this config.
export default defineConfig({
	testDir: 'e2e/live',

	use: {
		baseURL: 'http://localhost:8081'
	},

	// Distinct filename from playwright.config.ts's test-results/junit.xml —
	// this suite runs in a separate CI job (workstream 7's always-run `e2e`
	// job, not the mocked suite's `ts` job), but kept separate here too so
	// running both configs from the same working directory doesn't clobber
	// either report.
	reporter: [['list'], ['junit', { outputFile: 'test-results/junit-live.xml' }]]
});

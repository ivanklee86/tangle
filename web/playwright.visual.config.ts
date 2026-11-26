import { defineConfig } from '@playwright/test';

// Screenshot tests (project "visual") and the documentation screenshots
// (project "docs"). Both render pixels that only match in one environment,
// so run them through scripts/playwright-docker.sh, never directly on a host:
// fonts and anti-aliasing differ between machines and architectures.
// playwright.config.ts stays the host-safe mocked suite.
export default defineConfig({
	webServer: {
		command: 'npm run build && npm run preview -- --port 4174 --strictPort',
		port: 4174
	},

	use: {
		baseURL: 'http://localhost:4174',
		viewport: { width: 1440, height: 900 },
		deviceScaleFactor: 1,
		colorScheme: 'dark'
	},

	expect: {
		toHaveScreenshot: {
			animations: 'disabled',
			caret: 'hide'
		}
	},

	// One environment, so no platform suffix: e2e/visual/__screenshots__/<spec>/<name>.png
	snapshotPathTemplate: '{testDir}/__screenshots__/{testFilePath}/{arg}{ext}',

	projects: [
		{ name: 'visual', testDir: 'e2e/visual' },
		{ name: 'docs', testDir: 'e2e/docs' }
	],

	reporter: [['list'], ['junit', { outputFile: 'test-results/junit-visual.xml' }]]
});

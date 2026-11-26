import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { playwright } from '@vitest/browser-playwright';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			// Vitest runs this plugin in test mode, which writes .env.test's values
			// (an empty PUBLIC_BASE_URL) into the outDir's generated env modules.
			// Sharing .svelte-kit with a running `vite dev` meant a test run pointed
			// the dev server's API calls at itself, so tests get their own.
			outDir: process.env.VITEST ? '.svelte-kit-vitest' : '.svelte-kit',
			// SvelteKit 3 replaced $lib with #lib subpath imports; this keeps
			// $lib working until that migration lands (deprecated, warns on build).
			// https://github.com/ivanklee86/tangle/issues/258
			alias: { $lib: 'src/lib' },
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		})
	],

	test: {
		projects: [
			{
				extends: true,
				test: {
					name: 'node',
					environment: 'node',
					include: ['src/**/*.spec.{js,ts}'],
					exclude: ['src/**/*.svelte.test.{js,ts}']
				}
			},
			{
				extends: true,
				test: {
					name: 'browser',
					include: ['src/**/*.svelte.test.{js,ts}'],
					setupFiles: ['vitest-browser-svelte', './src/test/browser-setup.ts'],
					browser: {
						enabled: true,
						headless: true,
						provider: playwright(),
						instances: [{ browser: 'chromium' }]
					}
				}
			}
		]
	}
});

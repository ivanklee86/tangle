import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
	PUBLIC_BASE_URL: {
		public: true,
		static: true,
		description: 'Origin the SPA calls for /api. Empty means same-origin.'
	}
});

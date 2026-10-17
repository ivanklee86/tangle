import { vi } from 'vitest';

// Kit 3's dev env module reads globalThis.__sveltekit_dev, which Kit injects
// into the HTML it serves — Vitest's browser runner never gets it.
vi.mock('$app/env/public', () => ({ PUBLIC_BASE_URL: '' }));

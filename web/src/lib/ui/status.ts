import { CheckCircleSolid, CloseCircleSolid, ExclamationCircleSolid } from 'flowbite-svelte-icons';
import type { Component } from 'svelte';

interface StatusAppearance {
	icon: Component<{ class?: string }>;
	class: string;
}

const HEALTHY: StatusAppearance = {
	icon: CheckCircleSolid,
	class: 'text-green-500 dark:text-green-400'
};

const UNHEALTHY: StatusAppearance = {
	icon: CloseCircleSolid,
	class: 'text-red-500 dark:text-red-400'
};

const UNKNOWN: StatusAppearance = {
	icon: ExclamationCircleSolid,
	class: 'text-amber-500 dark:text-amber-400'
};

function statusAppearance(status: string): StatusAppearance {
	switch (status) {
		case 'Healthy':
		case 'Synced':
			return HEALTHY;
		case 'OutOfSync':
		case 'Degraded':
		case 'Missing':
			return UNHEALTHY;
		case 'Progressing':
		case 'Suspended':
		case 'Unknown':
			return UNKNOWN;
		default:
			return UNKNOWN;
	}
}

export { statusAppearance, type StatusAppearance };

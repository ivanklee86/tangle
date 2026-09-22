import {
	CheckCircleSolid,
	ClockSolid,
	CloseCircleSolid,
	ExclamationCircleSolid,
	QuestionCircleSolid
} from 'flowbite-svelte-icons';
import type { Component } from 'svelte';

type BadgeColor = 'green' | 'blue' | 'yellow' | 'red' | 'gray';

interface StatusAppearance {
	icon: Component<{ class?: string }>;
	/** Flowbite Badge `color`, so light and dark both come from Flowbite's own ramps. */
	color: BadgeColor;
	/**
	 * How much attention the status deserves, lowest first. Sorting by health
	 * or sync orders on this rather than on the status string, so "worst first"
	 * means worst first instead of alphabetical.
	 */
	severity: number;
	/** True for anything a human should look at — drives the "Needs attention" facet. */
	needsAttention: boolean;
}

// Argo CD's own colour vocabulary, so a status means the same thing in Tangle
// as it does in the Argo CD UI people came from. The important one is
// OutOfSync: it's drift, not breakage, so it's yellow — reserving red for
// Degraded and Missing, which are the states that actually want someone.
const HEALTHY: StatusAppearance = {
	icon: CheckCircleSolid,
	color: 'green',
	severity: 0,
	needsAttention: false
};

// Unknown and Suspended share a bucket, and neither counts as needing
// attention: Suspended is someone's deliberate pause, and grouping Unknown
// with it keeps "needs attention" meaning "Argo CD is telling us something is
// off" rather than "Argo CD didn't answer". They sort above Healthy so they
// don't hide at the bottom of a worst-first list.
const UNKNOWN: StatusAppearance = {
	icon: QuestionCircleSolid,
	color: 'gray',
	severity: 1,
	needsAttention: false
};

const DRIFTED: StatusAppearance = {
	icon: ExclamationCircleSolid,
	color: 'yellow',
	severity: 2,
	needsAttention: true
};

const PROGRESSING: StatusAppearance = {
	icon: ClockSolid,
	color: 'blue',
	severity: 3,
	needsAttention: true
};

const BROKEN: StatusAppearance = {
	icon: CloseCircleSolid,
	color: 'red',
	severity: 4,
	needsAttention: true
};

function statusAppearance(status: string): StatusAppearance {
	switch (status) {
		case 'Healthy':
		case 'Synced':
			return HEALTHY;
		case 'OutOfSync':
			return DRIFTED;
		case 'Progressing':
			return PROGRESSING;
		case 'Degraded':
		case 'Missing':
			return BROKEN;
		case 'Suspended':
		case 'Unknown':
			return UNKNOWN;
		default:
			// An unrecognised status is reported as-is rather than guessed at;
			// gray says "no opinion" instead of implying it's fine.
			return UNKNOWN;
	}
}

/** Severity of the worse of an application's two statuses. */
function statusSeverity(status: string): number {
	return statusAppearance(status).severity;
}

/**
 * Whether an application is worth a second look — either status being
 * attention-worthy is enough, since a Healthy application that's OutOfSync has
 * still drifted from what its manifests say.
 */
function needsAttention(health: string, syncStatus: string): boolean {
	return statusAppearance(health).needsAttention || statusAppearance(syncStatus).needsAttention;
}

export { statusAppearance, statusSeverity, needsAttention, type StatusAppearance, type BadgeColor };

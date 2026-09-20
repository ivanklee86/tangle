<script lang="ts">
	import { Heading } from 'flowbite-svelte';
	import { buildQuery } from '$lib/backend/url';
	import { ApplicationsForm, DiffsForm } from '$lib/ui/components';

	function goToApplications(labels: string, excludeLabels: string): void {
		// `searched: 'true'` survives even when labels/excludeLabels are both
		// empty (buildQuery drops empty values), so the Applications page can
		// tell "arrived from a submitted search" apart from a bare nav click.
		window.location.href = `/applications${buildQuery({ labels, excludeLabels, searched: 'true' })}`;
	}

	function goToDiffs(labels: string, excludeLabels: string, targetRef: string): void {
		window.location.href = `/diffs${buildQuery({ targetRef, labels, excludeLabels })}`;
	}
</script>

<svelte:head>
	<title>Tangle - Home</title>
</svelte:head>

<Heading tag="h1" class="sr-only">Tangle</Heading>

<div class="mt-6 grid grid-cols-1 gap-6 md:grid-cols-2 items-start">
	<ApplicationsForm onSubmit={goToApplications} />
	<DiffsForm onSubmit={goToDiffs} />
</div>

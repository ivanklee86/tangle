<script lang="ts">
	import {
		Button,
		Tabs,
		TabItem,
		Table,
		TableBody,
		TableBodyCell,
		TableBodyRow,
		TableHead,
		TableHeadCell
	} from 'flowbite-svelte';

	import { type ApplicationLinks, type ApplicationResponseStore } from '$lib/backend/data';
	import { filterOutZeroResults } from '$lib/ui/utils';
	import {
		nextSortState,
		sortIndicator as sortIndicatorFor,
		ariaSort as ariaSortFor,
		sortApplications,
		type SortKey,
		type SortState
	} from '$lib/ui/sort';
	import { ArgoCDHealthStatus, ArgoCDSyncStatus, ErrorAlert } from '$lib/ui/components';

	interface Props {
		applications: ApplicationResponseStore;
	}

	let { applications }: Props = $props();

	// flowbite-svelte 1.x dropped TableHeadCell's built-in `sort` prop/context, so
	// column sorting is reimplemented here (one independent sort state per tab).
	let sortState: Record<string, SortState | undefined> = $state({});

	function toggleSort(tabName: string, key: SortKey) {
		sortState[tabName] = nextSortState(sortState[tabName], key);
	}

	function sortIndicator(tabName: string, key: SortKey): string {
		return sortIndicatorFor(sortState[tabName], key);
	}

	function ariaSort(tabName: string, key: SortKey): 'ascending' | 'descending' | 'none' {
		return ariaSortFor(sortState[tabName], key);
	}

	function sortedApplications(
		tabName: string,
		applications: ApplicationLinks[]
	): ApplicationLinks[] {
		return sortApplications(applications, sortState[tabName]);
	}
</script>

{#if applications.error}
	<ErrorAlert message={applications.errorResponse?.error} />
{:else}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults(applications.response.results) as argoCDApplications, index (argoCDApplications.name)}
			<TabItem open={index === 0}>
				{#snippet titleSlot()}
					{argoCDApplications.name} ({argoCDApplications.applications.length})
				{/snippet}
				<Button href={argoCDApplications.link} target="_blank" class="mb-3">Take me there!</Button>
				<Table hoverable={true}>
					<TableHead>
						<TableHeadCell aria-sort={ariaSort(argoCDApplications.name, 'name')}>
							<button
								type="button"
								class="cursor-pointer select-none"
								onclick={() => toggleSort(argoCDApplications.name, 'name')}
								>Applications{sortIndicator(argoCDApplications.name, 'name')}</button
							>
						</TableHeadCell>
						<TableHeadCell aria-sort={ariaSort(argoCDApplications.name, 'health')}>
							<button
								type="button"
								class="cursor-pointer select-none"
								onclick={() => toggleSort(argoCDApplications.name, 'health')}
								>Health{sortIndicator(argoCDApplications.name, 'health')}</button
							>
						</TableHeadCell>
						<TableHeadCell aria-sort={ariaSort(argoCDApplications.name, 'syncStatus')}>
							<button
								type="button"
								class="cursor-pointer select-none"
								onclick={() => toggleSort(argoCDApplications.name, 'syncStatus')}
								>Sync Status{sortIndicator(argoCDApplications.name, 'syncStatus')}</button
							>
						</TableHeadCell>
					</TableHead>
					<TableBody class="divide-y">
						{#each sortedApplications(argoCDApplications.name, argoCDApplications.applications) as item (item.name)}
							<TableBodyRow>
								<TableBodyCell>
									<a href={item.url} target="_blank" rel="external" class="link-underline-primary"
										>{item.name}</a
									>
								</TableBodyCell>
								<TableBodyCell><ArgoCDHealthStatus healthStatus={item.health} /></TableBodyCell>
								<TableBodyCell><ArgoCDSyncStatus syncStatus={item.syncStatus} /></TableBodyCell>
							</TableBodyRow>
						{/each}
					</TableBody>
				</Table>
			</TabItem>
		{/each}
	</Tabs>
{/if}

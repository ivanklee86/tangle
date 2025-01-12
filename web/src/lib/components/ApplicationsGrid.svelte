<script>
	import {
		Button,
		Tabs,
		TabItem,
		Spinner,
		Table,
		TableBody,
		TableBodyCell,
		TableBodyRow,
		TableHead,
		TableHeadCell
	} from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';

	const labels = $page.url.searchParams.get('labels');
	var client = new TangleAPIClient();

	onMount(() => {
		client.getApplications(labels);
	});
</script>

{#if apiData}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each $apiData.results as argoCDApplications, index}
			<TabItem title={argoCDApplications.name} open={index === 0}>
				<span slot="title">{argoCDApplications.name}</span>
				<Button href={argoCDApplications.link} target="_blank" class="mb-3">Take me there!</Button>
				<br />
				<Table hoverable={true} items={argoCDApplications.applications}>
					<TableHead>
						<TableHeadCell sort={(a, b) => a.name.localeCompare(b.name)}>Applications</TableHeadCell
						>
					</TableHead>
					<TableBody tableBodyClass="divide-y">
						<TableBodyRow slot="row" let:item>
							<TableBodyCell>
								<a href={item.url} target="_blank" class="link-underline-primary">{item.name}</a>
							</TableBodyCell>
						</TableBodyRow>
					</TableBody>
				</Table>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<Spinner />
{/if}

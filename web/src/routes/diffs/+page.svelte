<script>
    import {
		Alert,
		Tabs,
		TabItem,
		Spinner
	} from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';
	import { ApplicationGrid } from '$lib/components';
	import AppManifests from '$lib/components/AppManifests.svelte';

	const labels = $page.url.searchParams.get('labels');
    const compareRef = $page.url.searchParams.get('compareRef');
	var client = new TangleAPIClient();

    let applications = [];

    let firstIndex = 0;
	onMount(() => {
		client.getApplications(labels).then(() => {
			if (!$apiData.error && $apiData.response.results.length > 0) {
				for (let i = 0; i < $apiData.response.results.length; i++) {
					if ($apiData.response.results[i].applications.length > 0) {
						firstIndex = i;
						break;
					}
				}
			}
		});
	});
</script>

{#if $apiData.error}
	<Alert color="none" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{$apiData.errorResponse?.error}
	</Alert>
{/if}

{#if apiData}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each $apiData.response.results as argoCDApplications, index}
			<TabItem
				title={argoCDApplications.name}
                open={index === firstIndex}
				disabled={argoCDApplications.applications.length === 0}
			>
                <Tabs>
                    {#each argoCDApplications.applications as application, appIndex}
                        <TabItem title={application.name} open={appIndex === 0}>
                            <span slot="title">{application.name}</span>
                            <br />
							<AppManifests
								argocd={argoCDApplications.name}
								appName={application.name}
								currentRef="main"
								compareRef="test_gitops"
							/>
                        </TabItem>
                    {/each}
                </Tabs>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<Spinner />
{/if}

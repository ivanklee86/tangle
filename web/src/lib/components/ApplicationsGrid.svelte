<script>

	import { Tabs, TabItem } from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';
	import { PUBLIC_BASE_URL } from '$env/static/public';

	const labels = $page.url.searchParams.get('labels');
	var url = `${PUBLIC_BASE_URL}/api/applications`;
	if (labels) {
		url = url + '?labels=' + labels;
	}

	onMount(async () => {
		fetch(url)
			.then((response) => response.json())
			.then((data) => {
				apiData.set(data);
			})
			.catch((error) => {
				console.log(error);
				return [];
			});
	});
</script>

<!-- {#if apiData}
	<Card>
		<TabContent>
			{#each $apiData.results as argoCDApplications, index}
				<TabPane tabId={argoCDApplications.name} tab={argoCDApplications.name} active={index === 0}>
					<Table bordered>
						<thead>
							<tr>
								<th
									>Applications (<a
										href={argoCDApplications.link}
										target="_blank"
										class="link-underline-primary">Link</a
									>)</th
								>
							</tr>
						</thead>
						<tbody>
							{#each argoCDApplications.applications as application}
								<tr>
									<td
										><a href={application.url} target="_blank" class="link-underline-primary"
											>{application.name}</a
										></td
									>
								</tr>
							{/each}
						</tbody>
					</Table>
				</TabPane>
			{/each}
		</TabContent>
	</Card>
{:else}
	<Spinner color="primary" />
{/if} -->

<Tabs>
	<TabItem open title="Profile">
	  <p class="text-sm text-gray-500 dark:text-gray-400">
		<b>Profile:</b>
		Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	  </p>
	</TabItem>
	<TabItem title="Settings">
	  <p class="text-sm text-gray-500 dark:text-gray-400">
		<b>Settings:</b>
		Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	  </p>
	</TabItem>
	<TabItem title="Users">
	  <p class="text-sm text-gray-500 dark:text-gray-400">
		<b>Users:</b>
		Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	  </p>
	</TabItem>
	<TabItem title="Dashboard">
	  <p class="text-sm text-gray-500 dark:text-gray-400">
		<b>Dashboard:</b>
		Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	  </p>
	</TabItem>
	<TabItem disabled>
	  <span slot="title" class="text-gray-400 dark:text-gray-500">Disabled</span>
	  <p class="text-sm text-gray-500 dark:text-gray-400">
		<b>Disabled:</b>
		Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
	  </p>
	</TabItem>
  </Tabs>

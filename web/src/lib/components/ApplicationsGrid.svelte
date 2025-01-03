<script>
	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';
	import { Card, TabContent, Table, TabPane, Spinner } from '@sveltestrap/sveltestrap';
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

{#if apiData}
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
{/if}

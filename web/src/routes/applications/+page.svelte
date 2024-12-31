<script>
	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';

	import {
		Card,
		Col,
		Container,
		Row,
		TabContent,
		Table,
		TabPane,
		Spinner
	} from '@sveltestrap/sveltestrap';

	import { ModeToggle } from '$lib/components';

	const labels = $page.url.searchParams.get('labels');

	import { PUBLIC_BASE_URL } from '$env/static/public';

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

<svelte:head>
	<link
		rel="stylesheet"
		href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.2/font/bootstrap-icons.css"
	/>
</svelte:head>

<Container fluid>
	<Row>
		<Col>
			<br />
			{#if apiData}
				<Card>
					<TabContent>
						{#each $apiData.results as argoCDApplications, index}
							{#if argoCDApplications.applications.length > 0}
								<TabPane
									tabId={argoCDApplications.name}
									tab={argoCDApplications.name}
									active={index === 0}
								>
									<Table bordered>
										<thead>
											<tr>
												<th>Applications</th>
											</tr>
										</thead>
										<tbody>
											{#each argoCDApplications.applications as application}
												<tr>
													<td><a href={application.url} target="_blank">{application.name}</a></td>
												</tr>
											{/each}
										</tbody>
									</Table>
								</TabPane>
							{/if}
						{/each}
					</TabContent>
				</Card>
			{:else}
				<Spinner color="primary" />
			{/if}
		</Col>
	</Row>
	<br />
</Container>

<ModeToggle />

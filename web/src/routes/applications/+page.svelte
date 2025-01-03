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

	import { ModeToggle, ApplicationGrid } from '$lib/components';
</script>

<Container fluid>
	<Row>
		<Col>
			<br />
			<ApplicationGrid />
		</Col>
	</Row>
	<br />
</Container>

<ModeToggle />

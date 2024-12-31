<script>
	import { Col, Container, Row } from '@sveltestrap/sveltestrap';
	import { ModeToggle } from '$lib/components';
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
			<p>Welcome to Tangle!</p>
			<br />
			<a href="/applications">Applications</a>
		</Col>
	</Row>
</Container>

<ModeToggle />

<script>
	import { Col, Container, Row } from '@sveltestrap/sveltestrap';
	import { ModeToggle } from '$lib/components';
</script>

<Container fluid>
	<Row>
		<Col>
			<p>Welcome to Tangle!</p>
			<br />
			<a href="/applications" class="link-underline-primary">Applications</a>
		</Col>
	</Row>
</Container>

<ModeToggle />

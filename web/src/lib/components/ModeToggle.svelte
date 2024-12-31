<script>
	// @ts-nocheck
	import {
		Col,
		Container,
		Row,
		Button,
		ThemeToggler,
		Icon,
		useColorMode
	} from '@sveltestrap/sveltestrap';

	useColorMode('dark');
</script>

<Container>
	<Row>
		<Col align="center">
			<ThemeToggler let:currentColorMode let:toggleColorMode>
				<Button color="primary" on:click={() => toggleColorMode()}>
					{#if currentColorMode === 'dark'}
						<Icon name="sun" />
					{:else}
						<Icon name="moon-stars-fill" />
					{/if}
				</Button>
			</ThemeToggler>
		</Col>
	</Row>
</Container>

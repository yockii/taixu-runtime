<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Taixu · Phase 0</title>
</svelte:head>

<main class="mx-auto max-w-7xl px-4 py-6">
	{@render children()}
</main>

<script lang="ts">
	import type { Values } from '$lib/api';
	import { t } from '$lib/i18n';
	import Bar from './Bar.svelte';

	let { values }: { values: Values | null } = $props();

	const sorted = $derived(
		values ? Object.entries(values.weights).sort((a, b) => b[1] - a[1]) : []
	);
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('values_title')}</h2>
	{#if sorted.length}
		<div class="space-y-2">
			{#each sorted as [name, w]}
				<Bar label={$t('val_' + name)} value={w} color="bg-glowsoft" />
			{/each}
		</div>
	{:else}
		<div class="text-sm text-dim">{$t('loading')}</div>
	{/if}
</div>

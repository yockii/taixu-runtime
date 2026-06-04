<script lang="ts">
	import type { Genome } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import Bar from './Bar.svelte';

	let { genome }: { genome: Genome | null } = $props();
	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('genome_title')}</h2>
	{#if genome}
		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			<Bar label={$t('curiosity')} value={genome.curiosity} color="bg-violet-500" />
			<Bar label={$t('sociability')} value={genome.sociability} color="bg-violet-500" />
			<Bar label={$t('creativity')} value={genome.creativity} color="bg-violet-500" />
			<Bar label={$t('persistence')} value={genome.persistence} color="bg-violet-500" />
			<Bar label={$t('risk_taking')} value={genome.risk_taking} color="bg-violet-500" />
			<Bar label={$t('empathy')} value={genome.empathy} color="bg-violet-500" />
		</div>
		<div class="mt-3 text-xs text-zinc-500">
			{$t('life_id')}: <code class="text-zinc-300">{genome.life_id}</code> · {$t('born_at')} {new Date(
				genome.born_at * 1000
			).toLocaleString(locale, { hour12: false })} · v{genome.genome_version}
		</div>
	{:else}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{/if}
</div>

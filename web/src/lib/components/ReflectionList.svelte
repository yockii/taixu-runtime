<script lang="ts">
	import { api, type Reflection, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { reflectionVer } from '$lib/stores';

	let items = $state<Reflection[]>([]);

	async function load() {
		const out = await api.reflections(30);
		items = out ?? [];
	}

	$effect(() => {
		$reflectionVer;
		load();
	});

	$effect(() => {
		const ti = setInterval(load, 30000);
		return () => clearInterval(ti);
	});

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('reflections_title')}</h2>
	{#if items.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_reflection')}</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto text-xs">
			{#each items as r (r.id)}
				<div class="border-l-2 border-violet-700 pl-3">
					<div class="flex items-baseline justify-between">
						<span class="font-mono text-zinc-500">#{r.id} · {r.kind}</span>
						<span class="text-zinc-500">{unixToDate(r.created_at, locale)}</span>
					</div>
					<div class="mt-0.5 text-zinc-300">{r.summary}</div>
					{#if r.insight}
						<div class="mt-0.5 text-emerald-400">▶ {r.insight}</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

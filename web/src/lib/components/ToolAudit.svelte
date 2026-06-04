<script lang="ts">
	import { api, type ToolAudit, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { toolVer } from '$lib/stores';

	let items = $state<ToolAudit[]>([]);

	async function load() {
		const out = await api.toolsAudit(30);
		items = out ?? [];
	}

	$effect(() => {
		$toolVer;
		load();
	});

	$effect(() => {
		const ti = setInterval(load, 60000);
		return () => clearInterval(ti);
	});

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('tools_title')}</h2>
	{#if items.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_tool')}</div>
	{:else}
		<div class="max-h-96 space-y-1 overflow-y-auto text-xs">
			{#each items as it (it.id)}
				<div class="flex items-baseline gap-2 border-b border-zinc-800 py-1">
					<span class="font-mono text-zinc-500">#{it.id}</span>
					<span class="{it.success ? 'text-emerald-400' : 'text-rose-400'} w-24 shrink-0">{it.tool_name}</span>
					<span class="w-16 shrink-0 tabular-nums text-zinc-500">{it.duration_ms}ms</span>
					<span class="flex-1 truncate text-zinc-300">{it.args_summary}</span>
					<span class="shrink-0 text-zinc-500">{unixToDate(it.started_at, locale)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

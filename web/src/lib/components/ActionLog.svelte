<script lang="ts">
	import { api, type ActionLog, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { actionVer } from '$lib/stores';

	let items = $state<ActionLog[]>([]);

	async function load() {
		const out = await api.actions(20);
		items = out ?? [];
	}

	$effect(() => {
		$actionVer;
		load();
	});

	$effect(() => {
		const ti = setInterval(load, 30000);
		return () => clearInterval(ti);
	});

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('actions_title')}</h2>
	{#if items.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_action')}</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto text-xs">
			{#each items as a (a.id)}
				<div class="border-l-2 {a.success ? 'border-emerald-700' : 'border-rose-700'} pl-3">
					<div class="flex items-baseline justify-between">
						<span class="font-mono text-zinc-500">
							#{a.id}
							<span class="ml-1 rounded bg-zinc-800 px-1 text-zinc-300">{a.kind}</span>
							{#if a.cycle_id > 0}· cycle {a.cycle_id}{/if}
							{#if a.goal_id > 0}· goal {a.goal_id}{/if}
						</span>
						<span class="text-zinc-500">{unixToDate(a.started_at, locale)}</span>
					</div>
					<div class="mt-0.5 font-mono text-zinc-400">{a.action}</div>
					<div class="mt-0.5 whitespace-pre-wrap text-zinc-200">{a.result}</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

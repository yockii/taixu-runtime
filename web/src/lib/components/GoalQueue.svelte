<script lang="ts">
	import { api, type Goal, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';

	let goals = $state<Goal[]>([]);
	let loading = $state(false);
	let tick = $state(0);

	async function load() {
		loading = true;
		try {
			const out = await api.goals('', 20);
			goals = out ?? [];
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		tick;
		load();
	});

	$effect(() => {
		const ti = setInterval(() => (tick += 1), 5000);
		return () => clearInterval(ti);
	});

	function statusColor(s: string): string {
		switch (s) {
			case 'pending':
				return 'text-amber-400';
			case 'active':
				return 'text-sky-400';
			case 'completed':
				return 'text-emerald-400';
			case 'rejected':
			case 'expired':
				return 'text-zinc-500';
			case 'failed':
				return 'text-rose-400';
			default:
				return 'text-zinc-300';
		}
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('goals_title')}</h2>
	{#if loading && goals.length === 0}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{:else if goals.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_goal')}</div>
	{:else}
		<div class="max-h-96 space-y-1 overflow-y-auto text-xs">
			{#each goals as g (g.id)}
				<div class="flex items-baseline gap-2 border-b border-zinc-800 py-1.5">
					<span class="font-mono text-zinc-500">#{g.id}</span>
					<span class="{statusColor(g.status)} w-20 shrink-0">{$t('status_' + g.status)}</span>
					<span class="w-32 shrink-0 truncate text-zinc-400">{$t('intent_' + g.intent)}</span>
					<span class="w-12 shrink-0 text-right tabular-nums text-zinc-300">{g.priority.toFixed(2)}</span>
					<span class="flex-1 truncate text-zinc-300">{g.payload}</span>
					<span class="shrink-0 text-zinc-500">{unixToDate(g.created_at, locale)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

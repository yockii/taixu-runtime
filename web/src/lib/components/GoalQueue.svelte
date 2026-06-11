<script lang="ts">
	import { api, type Goal, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { goalVer } from '$lib/stores';

	let goals = $state<Goal[]>([]);
	let loading = $state(false);

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
		$goalVer;
		load();
	});

	// 兜底定时刷新（防 SSE 断开后停更）
	$effect(() => {
		const ti = setInterval(load, 30000);
		return () => clearInterval(ti);
	});

	function statusColor(s: string): string {
		switch (s) {
			case 'pending':
				return 'text-[#ffc97a]';
			case 'active':
				return 'text-glow';
			case 'completed':
				return 'text-glow';
			case 'rejected':
				return 'text-[#ff7a96]';
			case 'expired':
				return 'text-dim';
			case 'failed':
				return 'text-[#ff7a96]';
			default:
				return 'text-fog';
		}
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('goals_title')}</h2>
	{#if loading && goals.length === 0}
		<div class="text-sm text-dim">{$t('loading')}</div>
	{:else if goals.length === 0}
		<div class="tempty">{$t('empty_goal')}</div>
	{:else}
		<div class="max-h-96 space-y-1 overflow-y-auto text-xs">
			{#each goals as g (g.id)}
				<div class="flex items-baseline gap-2 border-b border-line py-1.5">
					<span class="font-mono text-dim">#{g.id}</span>
					<span class="{statusColor(g.status)} w-20 shrink-0">{$t('status_' + g.status)}</span>
					<span class="w-32 shrink-0 truncate text-fog">{$t('intent_' + g.intent)}</span>
					<span class="w-12 shrink-0 text-right tabular-nums text-fog">{g.priority.toFixed(2)}</span>
					<span class="flex-1 truncate text-fog">{g.payload}</span>
					<span class="shrink-0 text-dim">{unixToDate(g.created_at, locale)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

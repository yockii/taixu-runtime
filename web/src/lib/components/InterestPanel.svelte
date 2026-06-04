<script lang="ts">
	import { t, lang } from '$lib/i18n';
	import { interestVer } from '$lib/stores';
	import { unixToDate } from '$lib/api';

	type Interest = {
		id: number;
		content: string;
		kind: string;
		strength: number;
		source_kind: string;
		source_ref?: string;
		created_at: number;
		last_seen_at: number;
		explored_count: number;
	};

	let items = $state<Interest[]>([]);

	async function load() {
		const r = await fetch('/api/interests?limit=20');
		if (r.ok) items = (await r.json()) ?? [];
	}

	$effect(() => {
		$interestVer;
		load();
	});

	$effect(() => {
		const ti = setInterval(load, 30000);
		return () => clearInterval(ti);
	});

	function kindColor(k: string): string {
		switch (k) {
			case 'skill':
				return 'text-amber-400';
			case 'knowledge':
				return 'text-cyan-400';
			case 'topic':
				return 'text-violet-400';
			case 'experience':
				return 'text-emerald-400';
			default:
				return 'text-zinc-400';
		}
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('interests_title')}</h2>
	{#if items.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_interest')}</div>
	{:else}
		<div class="max-h-72 space-y-1 overflow-y-auto text-xs">
			{#each items as i (i.id)}
				<div class="flex items-baseline gap-2 border-b border-zinc-800 py-1">
					<span class="font-mono text-zinc-500">#{i.id}</span>
					<span class="{kindColor(i.kind)} w-16 shrink-0">{$t('ikind_' + i.kind)}</span>
					<span class="w-12 shrink-0 text-right tabular-nums text-zinc-300">{i.strength.toFixed(2)}</span>
					<span class="flex-1 truncate text-zinc-200">{i.content}</span>
					<span class="shrink-0 text-zinc-500">{$t('explored_n')} {i.explored_count}</span>
					<span class="shrink-0 text-zinc-500">{unixToDate(i.last_seen_at, locale)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

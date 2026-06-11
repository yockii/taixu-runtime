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
		digest?: string;
		mastery: number;
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
				return 'text-[#ffc97a]';
			case 'knowledge':
				return 'text-glowsoft';
			case 'topic':
				return 'text-violet';
			case 'experience':
				return 'text-glow';
			default:
				return 'text-fog';
		}
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('interests_title')}</h2>
	{#if items.length === 0}
		<div class="tempty">{$t('empty_interest')}</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto text-xs">
			{#each items as i (i.id)}
				<div class="border-b border-line py-1">
					<div class="flex items-baseline gap-2">
						<span class="font-mono text-dim">#{i.id}</span>
						<span class="{kindColor(i.kind)} w-16 shrink-0">{$t('ikind_' + i.kind)}</span>
						<span class="flex-1 truncate text-bright">{i.content}</span>
						<span class="shrink-0 text-dim">{$t('explored_n')} {i.explored_count}</span>
					</div>
					<div class="mt-1 flex items-center gap-2">
						<!-- strength 条（绿）-->
						<span class="w-12 shrink-0 text-dim">{$t('strength_label')}</span>
						<div class="h-1.5 flex-1 rounded bg-white/5">
							<div class="h-full rounded bg-glow" style="width:{Math.round(i.strength * 100)}%"></div>
						</div>
						<span class="w-9 shrink-0 text-right tabular-nums text-fog">{i.strength.toFixed(2)}</span>
					</div>
					<div class="mt-0.5 flex items-center gap-2">
						<!-- mastery 条（琥珀）-->
						<span class="w-12 shrink-0 text-dim">{$t('mastery_label')}</span>
						<div class="h-1.5 flex-1 rounded bg-white/5">
							<div class="h-full rounded bg-[#ffc97a]" style="width:{Math.round(i.mastery * 100)}%"></div>
						</div>
						<span class="w-9 shrink-0 text-right tabular-nums text-fog">{i.mastery.toFixed(2)}</span>
					</div>
					{#if i.digest}
						<div class="mt-1 rounded bg-white/5 p-1.5 text-fog">{i.digest}</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<script lang="ts">
	import type { LifeState, MentalState } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import Bar from './Bar.svelte';

	let { life, mental }: { life: LifeState | null; mental: MentalState | null } = $props();

	function pct(v: number) {
		return Math.round(v * 100);
	}

	function color(v: number, inverted = false): string {
		const x = inverted ? 1 - v : v;
		if (x > 0.7) return 'bg-emerald-500';
		if (x > 0.4) return 'bg-amber-500';
		return 'bg-rose-500';
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('state_title')}</h2>

	{#if life && mental}
		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			<Bar label={$t('energy')} value={life.energy} color={color(life.energy)} />
			<Bar label={$t('competence')} value={life.competence} color={color(life.competence)} />
			<Bar label={$t('social_need')} value={life.social_need} color={color(life.social_need, true)} />
			<Bar label={$t('stress')} value={life.stress} color={color(life.stress, true)} />
			<Bar label={$t('confidence')} value={life.confidence} color={color(life.confidence)} />
			<Bar label={$t('stability')} value={life.stability} color={color(life.stability)} />
		</div>

		<hr class="my-4 border-zinc-800" />

		<div class="grid grid-cols-1 gap-3 md:grid-cols-3">
			<Bar label={$t('motivation')} value={mental.motivation} color={color(mental.motivation)} />
			<Bar label={$t('satisfaction')} value={mental.satisfaction} color={color(mental.satisfaction)} />
			<Bar label={$t('anxiety')} value={mental.anxiety} color={color(mental.anxiety, true)} />
		</div>

		<div class="mt-3 text-xs text-zinc-500">
			{$t('cap_label')} {pct(life.energy_daily_cap)}% · {$t('cap_used')} {pct(life.energy_used_today)}% · {$t('cap_reset_next')} {new Date(life.cap_reset_at * 1000).toLocaleString(locale, { hour12: false })}
		</div>
	{:else}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{/if}
</div>

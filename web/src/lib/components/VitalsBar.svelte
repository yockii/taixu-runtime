<script lang="ts">
	// 生命体征横幅：观察界面的「心电监护仪」。
	// 一眼读出：活着吗（lifecycle 呼吸灯）→ 第几循环 → 核心体征趋势（sparkline）→ 今日能量预算。
	import type { LifeState, MentalState } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { vitalHistory, type VitalFrame } from '$lib/stores';

	let {
		life,
		mental,
		lifecycleNow,
		lastTick
	}: {
		life: LifeState | null;
		mental: MentalState | null;
		lifecycleNow: string;
		lastTick: number;
	} = $props();

	const pct = (v: number) => Math.round(v * 100);

	// 体征语义色：正向指标高=青，中=琥珀，低=玫红；反向指标（stress/anxiety）取反
	function tone(v: number, inverted = false): string {
		const x = inverted ? 1 - v : v;
		if (x > 0.7) return 'var(--color-glow)';
		if (x > 0.4) return '#ffc97a';
		return '#ff7a96';
	}

	type Spec = { key: keyof VitalFrame; label: string; value: number; inverted: boolean };
	// 五大核心体征（带趋势线）；满意/焦虑直指生命体「过得好不好」，长跑观察主用
	const specs = $derived.by<Spec[]>(() => {
		if (!life || !mental) return [];
		return [
			{ key: 'energy', label: 'energy', value: life.energy, inverted: false },
			{ key: 'motivation', label: 'motivation', value: mental.motivation, inverted: false },
			{ key: 'satisfaction', label: 'satisfaction', value: mental.satisfaction, inverted: false },
			{ key: 'anxiety', label: 'anxiety', value: mental.anxiety, inverted: true },
			{ key: 'stress', label: 'stress', value: life.stress, inverted: true }
		];
	});

	// 次要体征（无趋势线小条）
	const minor = $derived.by(() => {
		if (!life) return [];
		return [
			{ label: 'competence', value: life.competence, inverted: false },
			{ label: 'social_need', value: life.social_need, inverted: true },
			{ label: 'confidence', value: life.confidence, inverted: false },
			{ label: 'stability', value: life.stability, inverted: false }
		];
	});

	// sparkline：90 帧 → svg polyline 坐标（viewBox 100x28，y 反转）
	function spark(key: keyof VitalFrame, frames: VitalFrame[]): string {
		if (frames.length < 2) return '';
		const n = frames.length;
		return frames
			.map((f, i) => `${((i / (n - 1)) * 100).toFixed(1)},${(26 - (f[key] as number) * 24).toFixed(1)}`)
			.join(' ');
	}

	const alive = $derived(lifecycleNow === 'Active');
	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<section class="card vitals reveal">
	<div class="flex flex-wrap items-center gap-x-5 gap-y-2">
		<div class="flex items-center gap-2">
			<span class="pulse-dot" class:off={!alive}></span>
			<span class="text-sm font-semibold text-bright">{$t('state_' + lifecycleNow)}</span>
		</div>
		{#if lastTick > 0}
			<span class="text-xs text-dim">{$t('cycle')} <span class="text-fog tabular-nums">{lastTick}</span></span>
		{/if}
		{#if life}
			<span class="ml-auto text-[11px] text-dim">
				{$t('cap_label')} {pct(life.energy_daily_cap)}% · {$t('cap_used')}
				<span class="tabular-nums" style="color:{tone(1 - life.energy_used_today / Math.max(life.energy_daily_cap, 0.01))}">{pct(life.energy_used_today)}%</span>
				· {$t('cap_reset_next')}
				{new Date(life.cap_reset_at * 1000).toLocaleTimeString(locale, { hour12: false, hour: '2-digit', minute: '2-digit' })}
			</span>
		{/if}
	</div>

	{#if specs.length}
		<div class="mt-4 grid grid-cols-2 gap-x-5 gap-y-3 sm:grid-cols-3 lg:grid-cols-5">
			{#each specs as s (s.label)}
				<div>
					<div class="mb-1 flex items-baseline justify-between text-[11px]">
						<span class="text-dim">{$t(s.label)}</span>
						<span class="tabular-nums" style="color:{tone(s.value, s.inverted)}">{pct(s.value)}%</span>
					</div>
					<div class="vbar">
						<div class="vbar-fill" style="width:{pct(s.value)}%; background:{tone(s.value, s.inverted)}"></div>
					</div>
					<svg class="mt-1 h-[28px] w-full opacity-80" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
						<polyline
							points={spark(s.key, $vitalHistory)}
							fill="none"
							stroke={tone(s.value, s.inverted)}
							stroke-width="1.4"
							stroke-linejoin="round"
							vector-effect="non-scaling-stroke"
						/>
					</svg>
				</div>
			{/each}
		</div>

		<div class="mt-2 grid grid-cols-2 gap-x-5 gap-y-2 border-t border-line pt-3 sm:grid-cols-4">
			{#each minor as m (m.label)}
				<div>
					<div class="mb-1 flex items-baseline justify-between text-[11px]">
						<span class="text-dim">{$t(m.label)}</span>
						<span class="text-fog tabular-nums">{pct(m.value)}%</span>
					</div>
					<div class="vbar">
						<div class="vbar-fill" style="width:{pct(m.value)}%; background:{tone(m.value, m.inverted)}"></div>
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="mt-3 text-xs text-dim">{$t('loading')}</div>
	{/if}
</section>

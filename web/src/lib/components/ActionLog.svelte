<script lang="ts">
	import { api, type ActionLog, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { actionVer } from '$lib/stores';
	import { locked } from '$lib/auth';
	import TokenGate from './TokenGate.svelte';

	// mode='dialogue' 展示对外言说（reflex），mode='action' 展示内在自主作为（deliberate）。
	// 二者分流：生命体「说的」与「做的」可背离，分开看才看得见这种差异。
	// 对话含用户原话 = 用户隐私，未授权时整块上锁（R87）。
	let { mode = 'action' }: { mode?: 'dialogue' | 'action' } = $props();

	let items = $state<ActionLog[]>([]);

	const isDialogue = $derived(mode === 'dialogue');
	const title = $derived(isDialogue ? $t('dialogue_title') : $t('action_title'));
	const emptyMsg = $derived(isDialogue ? $t('empty_dialogue') : $t('empty_action'));

	async function load() {
		const out = await api.actions(20, mode);
		items = out ?? [];
	}

	$effect(() => {
		$actionVer;
		void mode;
		if (isDialogue && $locked) {
			items = []; // 隐私：未授权不拉对话
			return;
		}
		load();
	});

	$effect(() => {
		const ti = setInterval(() => {
			if (isDialogue && $locked) return;
			load();
		}, 30000);
		return () => clearInterval(ti);
	});

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

{#snippet body()}
	{#if items.length === 0}
		<div class="tempty">{emptyMsg}</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto text-xs">
			{#each items as a (a.id)}
				{#if isDialogue}
					<!-- 对话：突出说出口的话（result），敷衍模式标灰 -->
					<div
						class="border-l-2 {a.kind === 'reflex_canned'
							? 'border-line'
							: 'border-glowsoft'} pl-3"
					>
						<div class="flex items-baseline justify-between">
							<span class="font-mono text-dim">
								#{a.id}
								{#if a.kind === 'reflex_canned'}<span
										class="ml-1 rounded bg-white/5 px-1 text-fog">{$t('canned_tag')}</span
									>{/if}
							</span>
							<span class="text-dim">{unixToDate(a.started_at, locale)}</span>
						</div>
						<div class="mt-0.5 whitespace-pre-wrap text-bright">{a.result}</div>
					</div>
				{:else}
					<!-- 行动：计划 + 执行轨迹 -->
					<div class="border-l-2 {a.success ? 'border-glow' : 'border-[#ff7a96]'} pl-3">
						<div class="flex items-baseline justify-between">
							<span class="font-mono text-dim">
								#{a.id}
								{#if a.cycle_id > 0}· cycle {a.cycle_id}{/if}
								{#if a.goal_id > 0}· goal {a.goal_id}{/if}
							</span>
							<span class="text-dim">{unixToDate(a.started_at, locale)}</span>
						</div>
						<div class="mt-0.5 font-mono text-fog">{a.action}</div>
						<div class="mt-0.5 whitespace-pre-wrap text-bright">{a.result}</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}
{/snippet}

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">
		{title}
		{#if isDialogue}<span class="ml-1 text-dim">· {$t('dialogue_hint')}</span>{/if}
	</h2>
	{#if isDialogue}
		<TokenGate>{@render body()}</TokenGate>
	{:else}
		{@render body()}
	{/if}
</div>

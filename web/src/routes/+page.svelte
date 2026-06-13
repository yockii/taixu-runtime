<script lang="ts">
	import { api } from '$lib/api';
	import type { Genome, LifeState, MentalState, Values } from '$lib/api';
	import { openStream } from '$lib/stream';
	import {
		goalVer,
		actionVer,
		reflectionVer,
		episodeVer,
		toolVer,
		interestVer,
		pushReflexReply,
		markReflexFinished,
		pushFeed,
		pushVitalFrame
	} from '$lib/stores';
	import { t } from '$lib/i18n';
	import { scale } from 'svelte/transition';
	import { authRequired } from '$lib/auth';
	import VitalsBar from '$lib/components/VitalsBar.svelte';
	import LiveFeed from '$lib/components/LiveFeed.svelte';
	import GenomePanel from '$lib/components/GenomePanel.svelte';
	import ValuesPanel from '$lib/components/ValuesPanel.svelte';
	import InjectForm from '$lib/components/InjectForm.svelte';
	import EpisodeStream from '$lib/components/EpisodeStream.svelte';
	import GoalQueue from '$lib/components/GoalQueue.svelte';
	import ReflectionList from '$lib/components/ReflectionList.svelte';
	import ActionLogPanel from '$lib/components/ActionLog.svelte';
	import DialoguePanel from '$lib/components/DialoguePanel.svelte';
	import ToolAuditPanel from '$lib/components/ToolAudit.svelte';
	import ConfigPanel from '$lib/components/ConfigPanel.svelte';
	import InterestPanel from '$lib/components/InterestPanel.svelte';
	import SkillPanel from '$lib/components/SkillPanel.svelte';
	import LangToggle from '$lib/components/LangToggle.svelte';
	import Genesis from '$lib/components/Genesis.svelte';

	let life = $state<LifeState | null>(null);
	let mental = $state<MentalState | null>(null);
	let genome = $state<Genome | null>(null);
	let values = $state<Values | null>(null);
	let lifecycleNow = $state('Unknown');
	let lastTick = $state(0);

	// 主区标签页。实况流默认页：打开面板第一眼=它此刻在干什么。
	type Tab = 'live' | 'goals' | 'dialogue' | 'actions' | 'reflections' | 'episodes' | 'tools';
	let tab = $state<Tab>('live');
	const tabs: { id: Tab; label: string }[] = [
		{ id: 'live', label: 'tab_live' },
		{ id: 'goals', label: 'tab_goals' },
		{ id: 'dialogue', label: 'tab_dialogue' },
		{ id: 'actions', label: 'tab_actions' },
		{ id: 'reflections', label: 'tab_reflections' },
		{ id: 'episodes', label: 'tab_episodes' },
		{ id: 'tools', label: 'tab_tools' }
	];

	// 诞生门控：裸 runtime 未配置时后端只 serve 诞生端点。先探 /api/genesis/status：
	// needs_config=true → 渲染宇宙诞生页；false 或正常模式下该端点 404(catch) → 进观测台。
	let genesisMode = $state<boolean | null>(null);
	let justBorn = $state(false); // 仅诞生→观测台首切放大特效；普通刷新不放
	let lifeName = $state(''); // 生命自我命名，左上角显示
	$effect(() => {
		api
			.genesisStatus()
			.then((s) => (genesisMode = s.needs_config))
			.catch(() => (genesisMode = false));
	});

	$effect(() => {
		if (genesisMode !== false) return; // 诞生模式 / 探测中：不拉观测数据
		api.state().then((s) => {
			life = s.life;
			mental = s.mental;
			if (s.name) lifeName = s.name;
			pushVitalFrame({
				energy: s.life.energy,
				stress: s.life.stress,
				motivation: s.mental.motivation,
				satisfaction: s.mental.satisfaction,
				anxiety: s.mental.anxiety,
				at: Math.floor(Date.now() / 1000)
			});
		});
		api.genome().then((g) => (genome = g));
		api.values().then((v) => (values = v));
		api.lifecycle().then((l) => (lifecycleNow = l.state));
		api.config().then((c) => authRequired.set(!!c.auth_required));
	});

	const clip = (s: string, n = 240) => (s && s.length > n ? s.slice(0, n) + '…' : (s ?? ''));

	$effect(() => {
		if (genesisMode !== false) return; // 诞生模式：不开实况流
		const close = openStream((ev) => {
			switch (ev.type) {
				case 'state':
					life = ev.life;
					mental = ev.mental;
					pushVitalFrame({
						energy: ev.life.energy,
						stress: ev.life.stress,
						motivation: ev.mental.motivation,
						satisfaction: ev.mental.satisfaction,
						anxiety: ev.mental.anxiety,
						at: Math.floor(Date.now() / 1000)
					});
					if (ev.reason) pushFeed('state', 'dim', 'fev_state', clip(ev.reason, 120));
					break;
				case 'lifecycle':
					lifecycleNow = ev.to_state;
					pushFeed('lifecycle', 'glow', 'fev_lifecycle', `${ev.from_state} → ${ev.to_state}${ev.reason ? ` · ${ev.reason}` : ''}`);
					break;
				case 'tick':
					lastTick = ev.cycle_id;
					break;
				case 'reflex_reply':
					pushReflexReply(ev.round, ev.content, ev.channel, ev.to, ev.created_at);
					pushFeed('reply', 'warm', 'fev_reply', clip(ev.content), ev.created_at);
					actionVer.update((n) => n + 1);
					interestVer.update((n) => n + 1);
					break;
				case 'reflex_finished':
					markReflexFinished();
					actionVer.update((n) => n + 1);
					break;
				case 'goal_enqueued':
					pushFeed('goal', 'cool', 'fev_goal', `${ev.intent} · p${ev.priority}${ev.payload ? ` · ${clip(ev.payload, 100)}` : ''}`);
					goalVer.update((n) => n + 1);
					break;
				case 'action_done':
					pushFeed('action', ev.success ? 'glow' : 'dim', ev.success ? 'fev_action' : 'fev_action_fail', clip(ev.action, 160), ev.started_at);
					actionVer.update((n) => n + 1);
					break;
				case 'reflection':
					pushFeed('reflection', 'violet', 'fev_reflection', `${ev.kind} · ${clip(ev.summary)}`);
					reflectionVer.update((n) => n + 1);
					break;
				case 'episode_sealed':
					pushFeed('episode', 'violet', 'fev_episode', `#${ev.episode_id} · ${clip(ev.summary)}`);
					episodeVer.update((n) => n + 1);
					break;
				case 'tool_audited':
					pushFeed('tool', 'cool', ev.success ? 'fev_tool' : 'fev_tool_fail', `${ev.tool_name} · ${ev.duration_ms}ms`);
					toolVer.update((n) => n + 1);
					break;
			}
		});
		return close;
	});
</script>

{#if genesisMode}
	<Genesis onborn={() => { justBorn = true; genesisMode = false; }} />
{:else if genesisMode === false}
<div class="born-in" in:scale={{ start: justBorn ? 0.5 : 1, opacity: justBorn ? 0 : 1, duration: justBorn ? 880 : 0, delay: justBorn ? 80 : 0 }}>
<header class="mb-5 flex items-baseline justify-between gap-4">
	<h1 class="text-xl font-bold text-bright">
		{#if lifeName}<span class="text-sm font-normal text-dim">{$t('brand')} · </span>{lifeName}{:else}{$t('title')}{/if}
	</h1>
	<LangToggle />
</header>

<VitalsBar {life} {mental} {lifecycleNow} {lastTick} />

<div class="mt-5 grid grid-cols-1 gap-5 lg:grid-cols-3">
	<!-- 移动端 order：先注入交互，再主区 -->
	<div class="order-2 lg:order-1 lg:col-span-2">
		<nav class="ttabs mb-4">
			{#each tabs as tb (tb.id)}
				<button class="ttab" class:on={tab === tb.id} onclick={() => (tab = tb.id)}>{$t(tb.label)}</button>
			{/each}
		</nav>

		{#if tab === 'live'}
			<LiveFeed />
		{:else if tab === 'goals'}
			<GoalQueue />
		{:else if tab === 'dialogue'}
			<DialoguePanel />
		{:else if tab === 'actions'}
			<ActionLogPanel mode="action" />
		{:else if tab === 'reflections'}
			<ReflectionList />
		{:else if tab === 'episodes'}
			<EpisodeStream />
		{:else}
			<ToolAuditPanel />
		{/if}
	</div>

	<div class="order-1 space-y-3 lg:order-2">
		<InjectForm />
		<details class="acc">
			<summary>{$t('interests_title')}</summary>
			<div class="acc-body"><InterestPanel /></div>
		</details>
		<details class="acc">
			<summary>{$t('skills_title')}</summary>
			<div class="acc-body"><SkillPanel /></div>
		</details>
		<details class="acc">
			<summary>{$t('genome_title')}</summary>
			<div class="acc-body"><GenomePanel {genome} /></div>
		</details>
		<details class="acc">
			<summary>{$t('values_title')}</summary>
			<div class="acc-body"><ValuesPanel {values} /></div>
		</details>
		<details class="acc">
			<summary>{$t('config_title')}</summary>
			<div class="acc-body"><ConfigPanel /></div>
		</details>
	</div>
</div>
</div>
{/if}

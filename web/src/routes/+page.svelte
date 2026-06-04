<script lang="ts">
	import { api } from '$lib/api';
	import type { Genome, LifeState, MentalState, Values } from '$lib/api';
	import { openStream } from '$lib/stream';
	import { t } from '$lib/i18n';
	import StatePanel from '$lib/components/StatePanel.svelte';
	import GenomePanel from '$lib/components/GenomePanel.svelte';
	import ValuesPanel from '$lib/components/ValuesPanel.svelte';
	import InjectForm from '$lib/components/InjectForm.svelte';
	import EpisodeStream from '$lib/components/EpisodeStream.svelte';
	import GoalQueue from '$lib/components/GoalQueue.svelte';
	import ReflectionList from '$lib/components/ReflectionList.svelte';
	import ActionLogPanel from '$lib/components/ActionLog.svelte';
	import ToolAuditPanel from '$lib/components/ToolAudit.svelte';
	import ConfigPanel from '$lib/components/ConfigPanel.svelte';
	import LangToggle from '$lib/components/LangToggle.svelte';

	let life = $state<LifeState | null>(null);
	let mental = $state<MentalState | null>(null);
	let genome = $state<Genome | null>(null);
	let values = $state<Values | null>(null);
	let lifecycleNow = $state('Unknown');
	let lastTick = $state(0);

	$effect(() => {
		api.state().then((s) => {
			life = s.life;
			mental = s.mental;
		});
		api.genome().then((g) => (genome = g));
		api.values().then((v) => (values = v));
		api.lifecycle().then((l) => (lifecycleNow = l.state));
	});

	$effect(() => {
		const close = openStream((ev) => {
			switch (ev.type) {
				case 'state':
					life = ev.life;
					mental = ev.mental;
					break;
				case 'lifecycle':
					lifecycleNow = ev.to_state;
					break;
				case 'tick':
					lastTick = ev.cycle_id;
					break;
				case 'speech':
					break;
			}
		});
		return close;
	});
</script>

<header class="mb-6 flex items-baseline justify-between gap-4">
	<h1 class="text-xl font-bold text-zinc-100">{$t('title')}</h1>
	<div class="flex items-center gap-3 text-xs text-zinc-500">
		<span><span class="text-emerald-400">●</span> {$t('state_' + lifecycleNow)}</span>
		{#if lastTick > 0}
			<span>{$t('cycle')} <span class="tabular-nums">{lastTick}</span></span>
		{/if}
		<LangToggle />
	</div>
</header>

<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
	<div class="space-y-4 lg:col-span-2">
		<StatePanel {life} {mental} />
		<GoalQueue />
		<ActionLogPanel />
		<EpisodeStream />
		<ReflectionList />
		<ToolAuditPanel />
	</div>
	<div class="space-y-4">
		<InjectForm />
		<GenomePanel {genome} />
		<ValuesPanel {values} />
		<ConfigPanel />
	</div>
</div>

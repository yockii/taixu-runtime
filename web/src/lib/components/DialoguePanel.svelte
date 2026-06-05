<script lang="ts">
	import { api, type DialogueTurn, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { actionVer } from '$lib/stores';
	import { locked } from '$lib/auth';
	import TokenGate from './TokenGate.svelte';

	// 完整对话：用户原话 + 生命体回复，分角色显示（修「你我不分」——原只显示生命体单边）。
	let turns = $state<DialogueTurn[]>([]);

	async function load() {
		try {
			turns = (await api.dialogue(30)) ?? [];
		} catch {
			turns = [];
		}
	}

	$effect(() => {
		$actionVer;
		if ($locked) {
			turns = [];
			return;
		}
		load();
	});

	$effect(() => {
		const ti = setInterval(() => {
			if (!$locked) load();
		}, 20000);
		return () => clearInterval(ti);
	});

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

{#snippet body()}
	{#if turns.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_dialogue')}</div>
	{:else}
		<div class="max-h-96 space-y-3 overflow-y-auto text-xs">
			{#each turns as turn, i (i)}
				{#if turn.role === 'user'}
					<!-- 用户（你）：右对齐 -->
					<div class="flex flex-col items-end">
						<span class="mb-0.5 text-[10px] text-zinc-500">{$t('speaker_user')}</span>
						<div class="max-w-[85%] rounded-lg rounded-tr-sm bg-zinc-700/60 px-3 py-1.5 whitespace-pre-wrap text-zinc-100">
							{turn.content}
						</div>
					</div>
				{:else}
					<!-- 生命体（它）：左对齐 -->
					<div class="flex flex-col items-start">
						<span class="mb-0.5 text-[10px] text-sky-500">{$t('speaker_life')}</span>
						<div class="max-w-[85%] rounded-lg rounded-tl-sm bg-sky-900/30 px-3 py-1.5 whitespace-pre-wrap text-zinc-100">
							{turn.content}
						</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}
{/snippet}

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">
		{$t('dialogue_title')}
		<span class="ml-1 text-zinc-600">· {$t('dialogue_hint')}</span>
	</h2>
	<TokenGate>{@render body()}</TokenGate>
</div>

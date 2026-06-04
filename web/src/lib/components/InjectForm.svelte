<script lang="ts">
	import { api } from '$lib/api';
	import { t } from '$lib/i18n';
	import { latestSpeech } from '$lib/stores';

	let text = $state('');
	let busy = $state(false);
	let lastID = $state('');
	let err = $state('');
	let waitFromID = $state<number | null>(null);

	async function send() {
		const tx = text.trim();
		if (!tx) return;
		busy = true;
		err = '';
		// 记录基线：下次 speech.id > baseline 即为本次回响
		waitFromID = $latestSpeech?.id ?? 0;
		try {
			const r = await api.injectExternal(tx);
			lastID = r.id;
			text = '';
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			busy = false;
		}
	}

	const reply = $derived(
		$latestSpeech && waitFromID !== null && $latestSpeech.id > waitFromID ? $latestSpeech : null
	);
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('inject_title')}</h2>
	<form
		class="space-y-2"
		onsubmit={(e) => {
			e.preventDefault();
			send();
		}}
	>
		<textarea
			class="h-20 w-full resize-none rounded border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm focus:border-zinc-500 focus:outline-none"
			placeholder={$t('inject_placeholder')}
			bind:value={text}
			disabled={busy}
		></textarea>
		<div class="flex items-center justify-between">
			<div class="text-xs text-zinc-500">
				{#if lastID}
					{$t('inject_last')}: <code class="text-emerald-400">{lastID}</code>
				{/if}
				{#if err}
					<span class="text-rose-400">{err}</span>
				{/if}
			</div>
			<button
				type="submit"
				class="rounded bg-emerald-600 px-4 py-1.5 text-sm font-medium hover:bg-emerald-500 disabled:opacity-50"
				disabled={busy || !text.trim()}
			>
				{busy ? $t('inject_busy') : $t('inject_send')}
			</button>
		</div>
	</form>

	{#if reply}
		<div class="mt-3 rounded border border-emerald-700/40 bg-emerald-900/20 p-3 text-sm">
			<div class="mb-1 text-xs font-semibold text-emerald-400">▶ {$t('reply_label')}</div>
			<div class="whitespace-pre-wrap text-zinc-100">{reply.content}</div>
		</div>
	{:else if waitFromID !== null && !reply}
		<div class="mt-3 text-xs text-zinc-500 italic">{$t('reply_waiting')}</div>
	{/if}
</div>

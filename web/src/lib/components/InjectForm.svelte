<script lang="ts">
	import { api } from '$lib/api';
	import { t } from '$lib/i18n';
	import { reflexReplies, reflexInProgress, resetReflexConversation } from '$lib/stores';
	import TokenGate from './TokenGate.svelte';

	let text = $state('');
	let busy = $state(false);
	let err = $state('');

	async function send() {
		const tx = text.trim();
		if (!tx) return;
		busy = true;
		err = '';
		resetReflexConversation(); // 清旧序列，进入新对话
		try {
			await api.injectExternal(tx);
			text = '';
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('inject_title')}</h2>
	<TokenGate>
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
	</TokenGate>

	{#if $reflexReplies.length > 0 || $reflexInProgress}
		<div class="mt-3 space-y-2">
			<div class="text-xs font-semibold text-emerald-400">▶ {$t('reply_label')}</div>
			{#each $reflexReplies as r (r.id)}
				<div class="rounded border border-emerald-700/40 bg-emerald-900/20 p-2 text-sm">
					<span class="mr-2 text-xs text-emerald-500">#{r.round}</span>
					<span class="whitespace-pre-wrap text-zinc-100">{r.content}</span>
				</div>
			{/each}
			{#if $reflexInProgress}
				<div class="text-xs italic text-zinc-500">{$t('reply_waiting')}</div>
			{/if}
		</div>
	{/if}
</div>

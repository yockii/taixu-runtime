<script lang="ts">
	import { api } from '$lib/api';
	import { t } from '$lib/i18n';

	let text = $state('');
	let busy = $state(false);
	let lastID = $state('');
	let err = $state('');

	async function send() {
		const tx = text.trim();
		if (!tx) return;
		busy = true;
		err = '';
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
</div>

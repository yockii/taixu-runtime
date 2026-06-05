<script lang="ts">
	// 写操作区块的「锁」：服务端要求令牌且本机还没填时，用居中的「输入访问令牌」占位
	// 替换掉内部交互内容；填入即解锁（store 响应式，一处填、处处解锁）。
	import { locked, saveToken } from '$lib/auth';
	import { t } from '$lib/i18n';

	let { children } = $props();
	let editing = $state(false);
	let value = $state('');

	function submit() {
		const v = value.trim();
		if (!v) return;
		saveToken(v);
		editing = false;
		value = '';
	}
</script>

{#if $locked}
	<div class="flex flex-col items-center justify-center gap-3 px-4 py-10 text-center">
		<div class="text-2xl opacity-60">🔒</div>
		{#if editing}
			<p class="text-xs text-zinc-500">{$t('access_token_hint')}</p>
			<div class="flex w-full max-w-xs gap-2">
				<!-- svelte-ignore a11y_autofocus -->
				<input
					type="password"
					bind:value
					autofocus
					placeholder={$t('access_token_ph')}
					onkeydown={(e) => e.key === 'Enter' && submit()}
					class="min-w-0 flex-1 rounded border border-zinc-700 bg-zinc-900 px-2 py-1.5 font-mono text-xs text-zinc-200 outline-none focus:border-violet-500"
				/>
				<button
					onclick={submit}
					class="shrink-0 rounded bg-violet-600 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-violet-500"
					>{$t('confirm')}</button
				>
			</div>
		{:else}
			<p class="text-sm text-zinc-500">{$t('locked_hint')}</p>
			<button
				onclick={() => (editing = true)}
				class="rounded-full border border-zinc-700 px-4 py-1.5 text-sm text-zinc-300 transition hover:border-violet-500 hover:text-white"
				>{$t('enter_token')}</button
			>
		{/if}
	</div>
{:else}
	{@render children()}
{/if}

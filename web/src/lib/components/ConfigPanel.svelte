<script lang="ts">
	import { api, type Config, getToken } from '$lib/api';
	import { saveToken as persistToken, token as tokenStore } from '$lib/auth';
	import { t } from '$lib/i18n';

	let cfg = $state<Config | null>(null);
	let token = $state(getToken());
	let saved = $state(false);

	$effect(() => {
		void $tokenStore; // 令牌变更后重新拉取（授权后才返回环境信息）
		api.config().then((c) => (cfg = c));
	});

	function saveToken() {
		persistToken(token.trim()); // 经 auth store → 响应式解锁所有受控区块
		saved = true;
		setTimeout(() => (saved = false), 1500);
	}
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('config_title')}</h2>
	{#if cfg}
		<div class="space-y-3 text-xs">
			{#if cfg.auth_required}
				<div class="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3">
					<div class="mb-1 flex items-center gap-1.5 font-semibold text-amber-300">
						🔒 {$t('access_token_title')}
					</div>
					<p class="mb-2 text-zinc-400">{$t('access_token_hint')}</p>
					<div class="flex gap-2">
						<input
							type="password"
							bind:value={token}
							placeholder={$t('access_token_ph')}
							class="min-w-0 flex-1 rounded border border-zinc-700 bg-zinc-900 px-2 py-1 font-mono text-zinc-200 outline-none focus:border-amber-500"
						/>
						<button
							onclick={saveToken}
							class="shrink-0 rounded bg-amber-600/80 px-3 py-1 font-medium text-white transition hover:bg-amber-600"
							>{saved ? $t('saved') : $t('save')}</button
						>
					</div>
				</div>
			{/if}
			{#if cfg.llm}
				<div>
					<div class="font-semibold text-zinc-400">{$t('llm_section')}</div>
					<div class="mt-1 grid grid-cols-2 gap-1 text-zinc-300">
						<span class="text-zinc-500">base_url</span><span class="font-mono break-all">{cfg.llm.base_url}</span>
						<span class="text-zinc-500">model</span><span class="font-mono">{cfg.llm.model}</span>
						<span class="text-zinc-500">temperature</span><span class="font-mono">{cfg.llm.temperature}</span>
						<span class="text-zinc-500">api_key</span><span class="font-mono break-all">{cfg.llm.api_key}</span>
					</div>
				</div>
			{/if}
			{#if cfg.feishu}
				<div>
					<div class="font-semibold text-zinc-400">{$t('feishu_section')}</div>
					<div class="mt-1 grid grid-cols-2 gap-1 text-zinc-300">
						<span class="text-zinc-500">app_id</span><span class="font-mono break-all">{cfg.feishu.app_id}</span>
						<span class="text-zinc-500">app_secret</span><span class="font-mono break-all">{cfg.feishu.app_secret}</span>
					</div>
				</div>
			{/if}
			{#if cfg.auth_required && !cfg.llm}
				<p class="text-xs text-zinc-600">{$t('config_locked_hint')}</p>
			{/if}
		</div>
	{:else}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{/if}
</div>

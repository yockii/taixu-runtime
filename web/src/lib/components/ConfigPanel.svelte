<script lang="ts">
	import { api, type Config } from '$lib/api';
	import { t } from '$lib/i18n';

	let cfg = $state<Config | null>(null);

	$effect(() => {
		api.config().then((c) => (cfg = c));
	});
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('config_title')}</h2>
	{#if cfg}
		<div class="space-y-3 text-xs">
			<div>
				<div class="font-semibold text-zinc-400">{$t('llm_section')}</div>
				<div class="mt-1 grid grid-cols-2 gap-1 text-zinc-300">
					<span class="text-zinc-500">base_url</span><span class="font-mono break-all">{cfg.llm.base_url}</span>
					<span class="text-zinc-500">model</span><span class="font-mono">{cfg.llm.model}</span>
					<span class="text-zinc-500">temperature</span><span class="font-mono">{cfg.llm.temperature}</span>
					<span class="text-zinc-500">api_key</span><span class="font-mono break-all">{cfg.llm.api_key}</span>
				</div>
			</div>
			<div>
				<div class="font-semibold text-zinc-400">{$t('feishu_section')}</div>
				<div class="mt-1 grid grid-cols-2 gap-1 text-zinc-300">
					<span class="text-zinc-500">app_id</span><span class="font-mono break-all">{cfg.feishu.app_id}</span>
					<span class="text-zinc-500">app_secret</span><span class="font-mono break-all">{cfg.feishu.app_secret}</span>
				</div>
			</div>
		</div>
	{:else}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{/if}
</div>

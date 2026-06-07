<script lang="ts">
	import { api, type Config, type EmbedStatus, getToken } from '$lib/api';
	import { saveToken as persistToken, token as tokenStore } from '$lib/auth';
	import { t } from '$lib/i18n';

	let cfg = $state<Config | null>(null);
	let token = $state(getToken());
	let saved = $state(false);

	// 嵌入增强记忆（面板自管 llama-server）
	let embed = $state<EmbedStatus | null>(null);
	let embedBusy = $state(false);
	let selQuant = $state('Q8_0');

	async function loadEmbed() {
		try {
			embed = await api.embedStatus();
			if (embed && !embedBusy) selQuant = embed.status.quant || selQuant;
		} catch {
			/* 忽略：嵌入状态非关键，轮询失败下次再来 */
		}
	}
	$effect(() => {
		loadEmbed();
		const id = setInterval(loadEmbed, 2000);
		return () => clearInterval(id);
	});

	async function toggleEmbed(on: boolean) {
		embedBusy = true;
		try {
			if (on) await api.embedEnable(selQuant);
			else await api.embedDisable();
			await loadEmbed();
		} catch (e) {
			console.error('embed toggle', e);
		} finally {
			embedBusy = false;
		}
	}

	function fmtMB(mb: number): string {
		return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : mb + ' MB';
	}
	function embedStateLabel(s: string): string {
		return $t('embed_state_' + s) || s;
	}

	let passphrase = $state('');
	let exporting = $state(false);
	let exportErr = $state('');

	// 勿扰时段本地编辑态（从 cfg 初始化）
	let qEnabled = $state(false);
	let qStart = $state(23);
	let qEnd = $state(8);
	let qTz = $state(0);
	let qSaved = $state(false);

	$effect(() => {
		void $tokenStore; // 令牌变更后重新拉取（授权后才返回环境信息）
		api.config().then((c) => {
			cfg = c;
			if (c.proactive_quiet) {
				qEnabled = c.proactive_quiet.enabled;
				qStart = c.proactive_quiet.start;
				qEnd = c.proactive_quiet.end;
				qTz = c.proactive_quiet.tz_offset_min;
			}
		});
	});

	async function saveQuiet() {
		await api.setQuiet({ enabled: qEnabled, start: qStart, end: qEnd, tz_offset_min: qTz });
		qSaved = true;
		setTimeout(() => (qSaved = false), 1500);
	}

	function saveToken() {
		persistToken(token.trim()); // 经 auth store → 响应式解锁所有受控区块
		saved = true;
		setTimeout(() => (saved = false), 1500);
	}

	async function doExport() {
		exportErr = '';
		if (passphrase.length < 8) {
			exportErr = $t('export_pass_short');
			return;
		}
		exporting = true;
		try {
			await api.exportLife(passphrase);
			passphrase = '';
		} catch (e) {
			exportErr = (e as Error).message;
		} finally {
			exporting = false;
		}
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

			{#if cfg.proactive_quiet}
				<div class="rounded-lg border border-zinc-700/60 bg-zinc-900/40 p-3">
					<label class="flex items-center gap-2 font-semibold text-zinc-300">
						<input type="checkbox" bind:checked={qEnabled} class="accent-violet-500" />
						🌙 {$t('quiet_enable')}
					</label>
					<p class="mt-1 mb-2 text-zinc-500">{$t('quiet_hint')}</p>
					<div class="flex flex-wrap items-center gap-2 text-zinc-300">
						<span>{$t('quiet_from')}</span>
						<input
							type="number"
							min="0"
							max="23"
							bind:value={qStart}
							class="w-14 rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-center font-mono outline-none focus:border-violet-500"
						/>
						<span>{$t('quiet_oclock')}</span>
						<span>{$t('quiet_to')}</span>
						<input
							type="number"
							min="0"
							max="23"
							bind:value={qEnd}
							class="w-14 rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-center font-mono outline-none focus:border-violet-500"
						/>
						<span>{$t('quiet_oclock')}</span>
					</div>
					<div class="mt-2 flex items-center gap-2">
						<input
							type="number"
							bind:value={qTz}
							class="w-20 rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-center font-mono text-zinc-300 outline-none focus:border-violet-500"
						/>
						<span class="text-zinc-500">{$t('quiet_tz')}</span>
						<button
							onclick={saveQuiet}
							class="ml-auto shrink-0 rounded bg-violet-600/80 px-3 py-1 font-medium text-white transition hover:bg-violet-600"
							>{qSaved ? $t('saved') : $t('save')}</button
						>
					</div>
				</div>
			{/if}

			{#if embed?.managed}
				{@const s = embed.status}
				<div class="rounded-lg border border-cyan-500/30 bg-cyan-500/5 p-3">
					<label class="flex items-center gap-2 font-semibold text-cyan-200">
						<input
							type="checkbox"
							checked={s.enabled}
							disabled={embedBusy || s.state === 'downloading' || s.state === 'starting'}
							onchange={(e) => toggleEmbed(e.currentTarget.checked)}
							class="accent-cyan-500"
						/>
						🧠 {$t('embed_enable')}
					</label>
					<p class="mt-1 mb-2 text-zinc-400">{$t('embed_hint')}</p>
					<p class="mb-2 text-amber-300/80">
						⚠ {$t('embed_mem_warn')
							.replace('{mem}', fmtMB(s.mem_estimate_mb))
							.replace('{size}', fmtMB(s.size_mb))}
					</p>

					<!-- 量化档选择（仅未启用时可改） -->
					{#if !s.enabled}
						<div class="mb-2 flex items-center gap-2 text-zinc-300">
							<span class="text-zinc-500">{$t('embed_quant_label')}</span>
							<select
								bind:value={selQuant}
								class="rounded border border-zinc-700 bg-zinc-900 px-2 py-1 font-mono text-zinc-200 outline-none focus:border-cyan-500"
							>
								{#each embed.quants as q}
									<option value={q.Name}>{q.Name} · {fmtMB(q.SizeMB)} · ~{fmtMB(q.MemMB)} RAM</option>
								{/each}
							</select>
							<span class="text-xs text-zinc-600">
								{s.model_present ? '✓ ' + $t('embed_model_present') : $t('embed_model_absent')}
							</span>
						</div>
					{/if}

					<!-- 状态行 -->
					<div class="flex items-center gap-2">
						<span
							class="inline-block h-2 w-2 shrink-0 rounded-full"
							class:bg-emerald-400={s.state === 'ready'}
							class:bg-amber-400={s.state === 'downloading' || s.state === 'starting'}
							class:bg-rose-500={s.state === 'error'}
							class:bg-zinc-600={s.state === 'disabled'}
						></span>
						<span class="text-zinc-300">
							{embedBusy
								? s.enabled
									? $t('embed_disabling')
									: $t('embed_enabling')
								: embedStateLabel(s.state)}
						</span>
						{#if s.quant && s.state !== 'disabled'}
							<span class="font-mono text-xs text-zinc-500">{s.quant}</span>
						{/if}
					</div>

					<!-- 下载进度条 -->
					{#if s.state === 'downloading' && s.download_total > 0}
						<div class="mt-2">
							<div class="h-2 w-full overflow-hidden rounded-full bg-zinc-800">
								<div
									class="h-full bg-cyan-500 transition-all"
									style="width: {s.download_pct.toFixed(1)}%"
								></div>
							</div>
							<div class="mt-1 text-right font-mono text-[10px] text-zinc-500">
								{fmtMB(Math.round(s.download_done / 1048576))} / {fmtMB(
									Math.round(s.download_total / 1048576)
								)} · {s.download_pct.toFixed(1)}%
							</div>
						</div>
					{/if}

					<!-- 向量覆盖 + 错误 -->
					{#if s.state === 'ready' || embed.coverage.embedded > 0}
						<div class="mt-2 text-xs text-zinc-500">
							{$t('embed_coverage')}: <span class="font-mono text-zinc-400"
								>{embed.coverage.embedded} / {embed.coverage.total}</span
							>
						</div>
					{/if}
					{#if s.err}
						<p class="mt-1 text-xs text-rose-400">{s.err}</p>
					{/if}
				</div>
			{/if}

			{#if !cfg.auth_required || cfg.llm}
				<div class="rounded-lg border border-violet-500/30 bg-violet-500/5 p-3">
					<div class="mb-1 font-semibold text-violet-300">📦 {$t('export_title')}</div>
					<p class="mb-2 text-zinc-400">{$t('export_hint')}</p>
					<div class="flex gap-2">
						<input
							type="password"
							bind:value={passphrase}
							placeholder={$t('export_pass_ph')}
							class="min-w-0 flex-1 rounded border border-zinc-700 bg-zinc-900 px-2 py-1 font-mono text-zinc-200 outline-none focus:border-violet-500"
						/>
						<button
							onclick={doExport}
							disabled={exporting}
							class="shrink-0 rounded bg-violet-600/80 px-3 py-1 font-medium text-white transition hover:bg-violet-600 disabled:opacity-50"
							>{exporting ? $t('exporting') : $t('export_btn')}</button
						>
					</div>
					{#if exportErr}
						<p class="mt-1 text-rose-400">{exportErr}</p>
					{/if}
					<p class="mt-2 text-amber-300/80">⚠ {$t('export_warn')}</p>
				</div>
			{/if}
		</div>
	{:else}
		<div class="text-sm text-zinc-500">{$t('loading')}</div>
	{/if}
</div>

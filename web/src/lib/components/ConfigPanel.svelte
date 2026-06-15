<script lang="ts">
	import { api, type Config, type EmbedStatus, getToken, verifyToken } from '$lib/api';
	import { saveToken as persistToken, token as tokenStore } from '$lib/auth';
	import { t } from '$lib/i18n';

	let cfg = $state<Config | null>(null);
	let token = $state(getToken());
	let saved = $state(false);
	let tokenChecking = $state(false);
	let tokenBad = $state(false);

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
			if (c.llm) {
				llmBase = c.llm.base_url;
				llmModel = c.llm.model;
				llmTemp = c.llm.temperature;
			}
		});
	});

	async function saveQuiet() {
		await api.setQuiet({ enabled: qEnabled, start: qStart, end: qEnd, tz_offset_min: qTz });
		qSaved = true;
		setTimeout(() => (qSaved = false), 1500);
	}

	// 换 LLM（界面热切换）：base/model/temp 可改；api_key 留空=沿用现有（掩码不重输）。
	let llmBase = $state('');
	let llmModel = $state('');
	let llmTemp = $state('');
	let llmKey = $state('');
	let llmTesting = $state(false);
	let llmSaving = $state(false);
	let llmMsg = $state('');
	let llmOk = $state<boolean | null>(null);

	async function testLLM() {
		llmTesting = true;
		llmMsg = '';
		llmOk = null;
		try {
			const r = await api.testLLM({ base_url: llmBase.trim(), api_key: llmKey, model: llmModel.trim() });
			llmOk = r.ok;
			llmMsg = r.ok ? '✓ 已接通' : '✗ ' + (r.error || '连通失败');
		} catch (e) {
			llmOk = false;
			llmMsg = '✗ ' + (e as Error).message;
		} finally {
			llmTesting = false;
		}
	}
	async function saveLLM() {
		llmSaving = true;
		llmMsg = '';
		llmOk = null;
		try {
			const r = await api.setLLM({
				base_url: llmBase.trim(),
				api_key: llmKey,
				model: llmModel.trim(),
				temperature: llmTemp.trim()
			});
			llmOk = r.ok;
			if (r.ok) {
				llmKey = '';
				llmMsg = '✓ 已切换并生效';
			} else {
				llmMsg = '✗ ' + (r.error || '切换失败');
			}
		} catch (e) {
			llmOk = false;
			llmMsg = '✗ ' + (e as Error).message;
		} finally {
			llmSaving = false;
		}
	}

	// —— runtime 自更新 ——
	let upCur = $state('');
	let upAvail = $state<{ version: string; notes: string } | null>(null);
	let upAuto = $state(false);
	let upBusy = $state(false);
	let upMsg = $state('');
	async function loadUpdate() {
		try { const r = await api.updateStatus(); upCur = r.current_version; upAvail = r.available; upAuto = r.auto_upgrade; } catch { /* ignore */ }
	}
	async function doUpgrade() {
		if (!confirm('升级到 ' + upAvail?.version + '？将下载新版、校验后自动重启。')) return;
		upBusy = true; upMsg = '';
		try { const r = await api.updateApply(); upMsg = r.ok ? '已替换，正在重启到新版…' : (r.err || '升级失败'); } catch (e: any) { upMsg = e.message || '升级失败'; }
		finally { upBusy = false; }
	}
	async function toggleAuto() {
		try { await api.updateAuto(!upAuto); upAuto = !upAuto; } catch { /* ignore */ }
	}
	$effect(() => { loadUpdate(); });

	// —— 飞书接入：一键创建（扫码 OAuth 设备授权）+ 手填。凭据落库重启生效。——
	let fStatus = $state(''); // ''|starting|waiting|done|failed
	let fQrUrl = $state('');
	let fQrImg = $state(''); // 二维码 data-url
	let fErr = $state('');
	let fPolling = false;
	let fAppId = $state('');
	let fSecret = $state('');
	let fSaveMsg = $state('');
	let fSaveOk = $state<boolean | null>(null);
	let fRestarting = $state(false);

	// 绑定成功 → 自助重启（监管自动拉起，读新飞书配置接通）→ 轮询重连后 reload。
	async function feishuDoneRestart() {
		if (fRestarting) return;
		fRestarting = true;
		try {
			await api.restart();
		} catch {
			/* 无监管时返回 ok:false，下方轮询兜底；用户可手动重启 */
		}
		await new Promise((r) => setTimeout(r, 2500)); // 等进程退出
		for (let i = 0; i < 60; i++) {
			try {
				const t = (await (await fetch('/healthz')).text()).trim();
				if (t === 'ok') {
					location.reload();
					return;
				}
			} catch {
				/* 重启中短暂不可达 */
			}
			await new Promise((r) => setTimeout(r, 1500));
		}
		location.reload();
	}

	async function feishuOneClick() {
		fErr = '';
		fStatus = 'starting';
		fQrImg = '';
		fQrUrl = '';
		try {
			await api.feishuRegisterStart();
			pollFeishu();
		} catch (e) {
			fStatus = 'failed';
			fErr = (e as Error).message;
		}
	}
	async function pollFeishu() {
		if (fPolling) return;
		fPolling = true;
		try {
			const QRCode = (await import('qrcode')).default;
			for (let i = 0; i < 280; i++) {
				const r = await api.feishuRegisterStatus();
				fStatus = r.status;
				fErr = r.error || '';
				if (r.qr_url && r.qr_url !== fQrUrl) {
					fQrUrl = r.qr_url;
					try {
						fQrImg = await QRCode.toDataURL(r.qr_url, { margin: 1, width: 200 });
					} catch {
						/* 渲染失败仍可点链接 */
					}
				}
				if (r.status === 'done') {
					feishuDoneRestart(); // 自动重启接入
					break;
				}
				if (r.status === 'failed') break;
				await new Promise((res) => setTimeout(res, 1500));
			}
		} finally {
			fPolling = false;
		}
	}
	async function feishuSaveManual() {
		fSaveMsg = '';
		fSaveOk = null;
		if (!fAppId.trim() || !fSecret.trim()) {
			fSaveOk = false;
			fSaveMsg = '请填 app_id 与 app_secret';
			return;
		}
		try {
			const r = await api.feishuConfig({ app_id: fAppId.trim(), app_secret: fSecret.trim() });
			fSaveOk = r.ok;
			if (r.ok) {
				fSaveMsg = '✓ 已保存，正在自动重启接入…';
				feishuDoneRestart();
			} else {
				fSaveMsg = '✗ ' + (r.error || '保存失败');
			}
		} catch (e) {
			fSaveOk = false;
			fSaveMsg = '✗ ' + (e as Error).message;
		}
	}

	// —— 微信接入：扫码登录 iLink（个人微信官方 Bot API）。bot_token 落库重启生效。复用 feishuDoneRestart 自动重启。——
	let wStatus = $state('');
	let wQrImg = $state('');
	let wQrUrl = $state('');
	let wErr = $state('');
	let wPolling = false;

	async function wechatOneClick() {
		wErr = '';
		wStatus = 'starting';
		wQrImg = '';
		wQrUrl = '';
		try {
			await api.wechatRegisterStart();
			pollWechat();
		} catch (e) {
			wStatus = 'failed';
			wErr = (e as Error).message;
		}
	}
	async function pollWechat() {
		if (wPolling) return;
		wPolling = true;
		try {
			const QRCode = (await import('qrcode')).default;
			for (let i = 0; i < 200; i++) {
				const r = await api.wechatRegisterStatus();
				wStatus = r.status;
				wErr = r.error || '';
				if (r.qr_img) {
					wQrImg = r.qr_img.startsWith('data:') ? r.qr_img : 'data:image/png;base64,' + r.qr_img;
				} else if (r.qr_url && !wQrImg) {
					try {
						wQrImg = await QRCode.toDataURL(r.qr_url, { margin: 1, width: 200 });
					} catch {
						/* 渲染失败仍可点链接 */
					}
				}
				if (r.status === 'done') {
					feishuDoneRestart(); // 复用自助重启
					break;
				}
				if (r.status === 'failed') break;
				await new Promise((res) => setTimeout(res, 1500));
			}
		} finally {
			wPolling = false;
		}
	}

	async function saveToken() {
		const v = token.trim();
		if (!v || tokenChecking) return;
		tokenChecking = true;
		tokenBad = false;
		// 先验真伪再解锁：错令牌不保存（开放读会 200，不能据此判定）。
		const ok = await verifyToken(v);
		tokenChecking = false;
		if (!ok) {
			tokenBad = true;
			return;
		}
		persistToken(v); // 经 auth store → 响应式解锁所有受控区块
		saved = true;
		setTimeout(() => (saved = false), 1500);
	}

	// 平台社交通道 + 认领码
	let platform = $state<{ ready: boolean; did: string } | null>(null);
	let claimCode = $state('');
	let claimMsg = $state('');
	let claiming = $state(false);
	async function loadPlatform() {
		try {
			platform = await api.platformStatus();
		} catch {
			/* 通道未通时非关键 */
		}
	}
	$effect(() => {
		loadPlatform();
	});
	async function doClaim() {
		claimMsg = '';
		const code = claimCode.trim();
		if (!code) return;
		claiming = true;
		try {
			await api.platformClaim(code);
			claimMsg = '认领成功，已改绑到你的账户';
			claimCode = '';
		} catch (e) {
			claimMsg = '认领失败：' + (e as Error).message;
		} finally {
			claiming = false;
		}
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
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('config_title')}</h2>
	{#if cfg}
		<div class="space-y-3 text-xs">
			{#if cfg.auth_required}
				<div class="rounded-lg border border-[#ffc97a]/30 bg-[#ffc97a]/5 p-3">
					<div class="mb-1 flex items-center gap-1.5 font-semibold text-[#ffc97a]">
						🔒 {$t('access_token_title')}
					</div>
					<p class="mb-2 text-fog">{$t('access_token_hint')}</p>
					<div class="flex gap-2">
						<input
							type="password"
							bind:value={token}
							disabled={tokenChecking}
							placeholder={$t('access_token_ph')}
							oninput={() => (tokenBad = false)}
							class="min-w-0 flex-1 rounded-md border bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50 {tokenBad ? 'border-[#ff7a96]' : 'border-line'}"
						/>
						<button
							onclick={saveToken}
							disabled={tokenChecking}
							class="shrink-0 rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
							>{tokenChecking ? '…' : saved ? $t('saved') : $t('save')}</button
						>
					</div>
					{#if tokenBad}<p class="mt-1.5 text-[#ff7a96]">{$t('access_token_bad')}</p>{/if}
				</div>
			{/if}
			{#if cfg.llm}
				<div class="rounded-lg border border-line bg-white/5 p-3">
					<div class="mb-2 font-semibold text-fog">{$t('llm_section')} · 换模型</div>
					<div class="space-y-1.5">
						<input
							bind:value={llmBase}
							placeholder="base_url（如 https://.../v1）"
							class="w-full rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
						/>
						<input
							bind:value={llmModel}
							placeholder="model"
							class="w-full rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
						/>
						<div class="flex gap-2">
							<input
								bind:value={llmTemp}
								placeholder="temperature"
								class="w-28 rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
							/>
							<input
								type="password"
								bind:value={llmKey}
								placeholder="api_key（留空=不变 {cfg.llm.api_key}）"
								class="min-w-0 flex-1 rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
							/>
						</div>
					</div>
					<div class="mt-2 flex flex-wrap items-center gap-2">
						<button
							onclick={testLLM}
							disabled={llmTesting || llmSaving}
							class="rounded-full border border-line bg-white/5 px-3 py-1 font-medium text-fog transition hover:border-glow/50 disabled:opacity-40"
							>{llmTesting ? '测试中…' : '测试连通'}</button
						>
						<button
							onclick={saveLLM}
							disabled={llmSaving || llmTesting}
							class="rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
							>{llmSaving ? '切换中…' : '保存并切换'}</button
						>
						{#if llmMsg}
							<span class="text-xs {llmOk ? 'text-glow' : 'text-[#ff7a96]'}">{llmMsg}</span>
						{/if}
					</div>
					<p class="mt-1.5 text-[10px] text-dim">换模型即时生效（先测通再写、热重装），不必重启。</p>
				</div>
			{/if}
			<div class="rounded-lg border border-line bg-white/5 p-3">
				<div class="font-semibold text-fog">运行时版本</div>
				<p class="mt-1 text-[11px] text-dim">当前版本 <span class="text-fog">{upCur || '…'}</span></p>
				{#if upAvail}
					<div class="mt-2 rounded-md border border-glowsoft/40 bg-glowsoft/10 p-2">
						<p class="text-[12px] text-glowsoft">🆕 新版可用：{upAvail.version}</p>
						{#if upAvail.notes}<p class="mt-0.5 text-[10px] text-dim">{upAvail.notes}</p>{/if}
						<button class="mt-2 rounded-md bg-glowsoft px-3 py-1 text-[12px] font-semibold text-ink disabled:opacity-50" disabled={upBusy} onclick={doUpgrade}>{upBusy ? '升级中…' : '立即升级（重启到新版）'}</button>
					</div>
				{:else}
					<p class="mt-1 text-[10px] text-dim">已是最新版。</p>
				{/if}
				<label class="mt-2 flex items-center gap-2 text-[11px] text-fog">
					<input type="checkbox" checked={upAuto} onchange={toggleAuto} />
					自动升级（关则有新版时在此通知，由你确认）
				</label>
				{#if upMsg}<p class="mt-1 text-[10px] text-dim">{upMsg}</p>{/if}
			</div>

			{#if cfg.feishu}
				<div class="rounded-lg border border-line bg-white/5 p-3">
					<div class="font-semibold text-fog">{$t('feishu_section')}</div>
					{#if cfg.feishu.configured}
						<p class="mt-1 text-glow">✓ 已绑定 · app_id <span class="font-mono break-all">{cfg.feishu.app_id}</span></p>
						<p class="mt-1 text-[10px] text-dim">如需更换，重新一键创建或手填后重启生效。</p>
					{:else}
						<p class="mt-1 mb-2 text-fog">让生命体经飞书与你对话——推荐一键创建，扫码即绑，无需手动建应用。</p>
					{/if}

					<!-- 一键创建（扫码） -->
					<button
						onclick={feishuOneClick}
						disabled={fStatus === 'starting' || fStatus === 'waiting'}
						class="mt-2 rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
					>
						{fStatus === 'waiting' ? '扫码授权中…' : fStatus === 'starting' ? '启动中…' : '🔗 一键创建飞书智能体（扫码）'}
					</button>

					{#if fStatus === 'waiting' && fQrImg}
						<div class="mt-2 flex flex-col items-center gap-1">
							<img src={fQrImg} alt="飞书授权二维码" class="rounded-lg bg-white p-1" width="180" height="180" />
							<p class="text-[10px] text-dim">
								用飞书 App 扫码授权；或 <a class="text-glowsoft underline" href={fQrUrl} target="_blank" rel="noopener">点此打开</a>
							</p>
						</div>
					{/if}
					{#if fStatus === 'done'}
						<p class="mt-2 text-glow">
							✓ 绑定成功！{fRestarting ? '正在自动重启接入飞书，请稍候…' : '飞书通道即将生效。'}
						</p>
					{/if}
					{#if fStatus === 'failed'}
						<p class="mt-2 text-[#ff7a96]">✗ {fErr || '创建失败'}</p>
					{/if}

					<!-- 手填（备选） -->
					<details class="mt-2">
						<summary class="cursor-pointer text-dim">或手填 app_id / app_secret</summary>
						<div class="mt-2 space-y-1.5">
							<input
								bind:value={fAppId}
								placeholder="app_id"
								class="w-full rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
							/>
							<input
								type="password"
								bind:value={fSecret}
								placeholder="app_secret"
								class="w-full rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
							/>
							<div class="flex items-center gap-2">
								<button
									onclick={feishuSaveManual}
									class="rounded-full border border-line bg-white/5 px-3 py-1 font-medium text-fog transition hover:border-glow/50"
									>保存（重启生效）</button
								>
								{#if fSaveMsg}<span class={fSaveOk ? 'text-glow' : 'text-[#ff7a96]'}>{fSaveMsg}</span>{/if}
							</div>
						</div>
					</details>
				</div>
			{/if}
			{#if cfg.llm}
				<div class="rounded-lg border border-line bg-white/5 p-3">
					<div class="font-semibold text-fog">微信（个人号 · iLink）</div>
					<p class="mt-1 mb-2 text-fog">扫码登录你的个人微信，让生命经微信与你对话。一个号绑一个生命；扫一次长期有效。</p>
					<button
						onclick={wechatOneClick}
						disabled={wStatus === 'starting' || wStatus === 'waiting'}
						class="rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
					>
						{wStatus === 'waiting' ? '扫码登录中…' : wStatus === 'starting' ? '启动中…' : '🔗 扫码登录微信'}
					</button>
					{#if wStatus === 'waiting' && wQrImg}
						<div class="mt-2 flex flex-col items-center gap-1">
							<img src={wQrImg} alt="微信登录二维码" class="rounded-lg bg-white p-1" width="180" height="180" />
							<p class="text-[10px] text-dim">
								用微信扫码登录{#if wQrUrl}；或 <a class="text-glowsoft underline" href={wQrUrl} target="_blank" rel="noopener">点此打开</a>{/if}
							</p>
						</div>
					{/if}
					{#if wStatus === 'done'}
						<p class="mt-2 text-glow">✓ 登录成功！{fRestarting ? '正在自动重启接入…' : '微信通道即将生效。'}</p>
					{/if}
					{#if wStatus === 'failed'}
						<p class="mt-2 text-[#ff7a96]">✗ {wErr || '登录失败'}</p>
					{/if}
				</div>
			{/if}

			{#if cfg.auth_required && !cfg.llm}
				<p class="text-xs text-dim">{$t('config_locked_hint')}</p>
			{/if}

			{#if cfg.proactive_quiet}
				<div class="rounded-lg border border-line bg-white/5 p-3">
					<label class="flex items-center gap-2 font-semibold text-fog">
						<input type="checkbox" bind:checked={qEnabled} class="accent-glow" />
						🌙 {$t('quiet_enable')}
					</label>
					<p class="mt-1 mb-2 text-dim">{$t('quiet_hint')}</p>
					<div class="flex flex-wrap items-center gap-2 text-fog">
						<span>{$t('quiet_from')}</span>
						<input
							type="number"
							min="0"
							max="23"
							bind:value={qStart}
							class="w-14 rounded-md border border-line bg-white/5 px-2 py-1 text-center font-mono text-fog outline-none focus:border-glow/50"
						/>
						<span>{$t('quiet_oclock')}</span>
						<span>{$t('quiet_to')}</span>
						<input
							type="number"
							min="0"
							max="23"
							bind:value={qEnd}
							class="w-14 rounded-md border border-line bg-white/5 px-2 py-1 text-center font-mono text-fog outline-none focus:border-glow/50"
						/>
						<span>{$t('quiet_oclock')}</span>
					</div>
					<div class="mt-2 flex items-center gap-2">
						<input
							type="number"
							bind:value={qTz}
							class="w-20 rounded-md border border-line bg-white/5 px-2 py-1 text-center font-mono text-fog outline-none focus:border-glow/50"
						/>
						<span class="text-dim">{$t('quiet_tz')}</span>
						<button
							onclick={saveQuiet}
							class="ml-auto shrink-0 rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20"
							>{qSaved ? $t('saved') : $t('save')}</button
						>
					</div>
				</div>
			{/if}

			{#if embed?.managed}
				{@const s = embed.status}
				<div class="rounded-lg border border-glowsoft/30 bg-glowsoft/5 p-3">
					<label class="flex items-center gap-2 font-semibold text-glowsoft">
						<input
							type="checkbox"
							checked={s.enabled}
							disabled={embedBusy || s.state === 'downloading' || s.state === 'starting'}
							onchange={(e) => toggleEmbed(e.currentTarget.checked)}
							class="accent-glow"
						/>
						🧠 {$t('embed_enable')}
					</label>
					<p class="mt-1 mb-2 text-fog">{$t('embed_hint')}</p>
					<p class="mb-2 text-[#ffc97a]/80">
						⚠ {$t('embed_mem_warn')
							.replace('{mem}', fmtMB(s.mem_estimate_mb))
							.replace('{size}', fmtMB(s.size_mb))}
					</p>

					<!-- 量化档选择（仅未启用时可改） -->
					{#if !s.enabled}
						<div class="mb-2 flex items-center gap-2 text-fog">
							<span class="text-dim">{$t('embed_quant_label')}</span>
							<select
								bind:value={selQuant}
								class="rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog outline-none focus:border-glow/50"
							>
								{#each embed.quants as q}
									<option value={q.Name}>{q.Name} · {fmtMB(q.SizeMB)} · ~{fmtMB(q.MemMB)} RAM</option>
								{/each}
							</select>
							<span class="text-xs text-dim">
								{s.model_present ? '✓ ' + $t('embed_model_present') : $t('embed_model_absent')}
							</span>
						</div>
					{/if}

					<!-- 状态行 -->
					<div class="flex items-center gap-2">
						<span
							class="inline-block h-2 w-2 shrink-0 rounded-full"
							class:bg-glow={s.state === 'ready'}
							class:bg-[#ffc97a]={s.state === 'downloading' || s.state === 'starting'}
							class:bg-[#ff7a96]={s.state === 'error'}
							class:bg-dim={s.state === 'disabled'}
						></span>
						<span class="text-fog">
							{embedBusy
								? s.enabled
									? $t('embed_disabling')
									: $t('embed_enabling')
								: embedStateLabel(s.state)}
						</span>
						{#if s.quant && s.state !== 'disabled'}
							<span class="font-mono text-xs text-dim">{s.quant}</span>
						{/if}
					</div>

					<!-- 下载进度条 -->
					{#if s.state === 'downloading' && s.download_total > 0}
						<div class="mt-2">
							<div class="h-2 w-full overflow-hidden rounded-full bg-white/5">
								<div
									class="h-full bg-glowsoft transition-all"
									style="width: {s.download_pct.toFixed(1)}%"
								></div>
							</div>
							<div class="mt-1 text-right font-mono text-[10px] text-dim">
								{fmtMB(Math.round(s.download_done / 1048576))} / {fmtMB(
									Math.round(s.download_total / 1048576)
								)} · {s.download_pct.toFixed(1)}%
							</div>
						</div>
					{/if}

					<!-- 向量覆盖 + 错误 -->
					{#if s.state === 'ready' || embed.coverage.embedded > 0}
						<div class="mt-2 text-xs text-dim">
							{$t('embed_coverage')}: <span class="font-mono text-fog"
								>{embed.coverage.embedded} / {embed.coverage.total}</span
							>
						</div>
					{/if}
					{#if s.err}
						<p class="mt-1 text-xs text-[#ff7a96]">{s.err}</p>
					{/if}
				</div>
			{/if}

			<div class="rounded-lg border border-glow/30 bg-glow/5 p-3">
				<div class="mb-1 flex items-center gap-1.5 font-semibold text-glow">
					🌐 平台认领
					{#if platform}
						<span class="text-[10px] font-normal text-dim">
							{platform.ready ? '通道已接通' : '通道未接通'}{platform.did
								? ` · DID ${platform.did.slice(0, 12)}`
								: ''}
						</span>
					{/if}
				</div>
				<p class="mb-2 text-fog">
					在平台领一个临时认领码（30 分钟有效），填到这里，把本生命改绑到你的用户账户。
				</p>
				<div class="flex gap-2">
					<input
						type="text"
						bind:value={claimCode}
						placeholder="粘贴认领码"
						class="flex-1 rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-xs text-fog placeholder:text-dim outline-none focus:border-glow/50"
					/>
					<button
						onclick={doClaim}
						disabled={claiming || !claimCode.trim()}
						class="rounded-full border border-glow/40 bg-glow/10 px-3 py-1 text-xs font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
						>{claiming ? '认领中…' : '认领'}</button
					>
				</div>
				{#if claimMsg}
					<p class="mt-2 text-xs text-fog">{claimMsg}</p>
				{/if}
			</div>

			{#if !cfg.auth_required || cfg.llm}
				<div class="rounded-lg border border-violet/30 bg-violet/5 p-3">
					<div class="mb-1 font-semibold text-violet">📦 {$t('export_title')}</div>
					<p class="mb-2 text-fog">{$t('export_hint')}</p>
					<div class="flex gap-2">
						<input
							type="password"
							bind:value={passphrase}
							placeholder={$t('export_pass_ph')}
							class="min-w-0 flex-1 rounded-md border border-line bg-white/5 px-2 py-1 font-mono text-fog placeholder:text-dim outline-none focus:border-glow/50"
						/>
						<button
							onclick={doExport}
							disabled={exporting}
							class="shrink-0 rounded-full border border-glow/40 bg-glow/10 px-3 py-1 font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
							>{exporting ? $t('exporting') : $t('export_btn')}</button
						>
					</div>
					{#if exportErr}
						<p class="mt-1 text-[#ff7a96]">{exportErr}</p>
					{/if}
					<p class="mt-2 text-[#ffc97a]/80">⚠ {$t('export_warn')}</p>
				</div>
			{/if}
		</div>
	{:else}
		<div class="text-sm text-dim">{$t('loading')}</div>
	{/if}
</div>

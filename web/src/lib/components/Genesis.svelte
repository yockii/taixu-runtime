<script lang="ts">
	import { api } from '$lib/api';
	import { saveToken } from '$lib/auth';

	// 宇宙基调诞生页：裸 runtime 未配置时显示。母语 + LLM + 守护令牌 → 测连通 → 孕育 → 星核坍缩动画 → 进观测台。
	// 平台账号不在此填（诞生后在观测台「认领」）。
	// onborn：诞生完成回调——父组件据此切到观测台（SPA 内切换，不 reload，便于切页放大特效）。
	let { onborn }: { onborn?: () => void } = $props();

	const LANGS = [
		{ c: 'zh', n: '中文' },
		{ c: 'en', n: 'English' },
		{ c: 'ja', n: '日本語' },
		{ c: 'ko', n: '한국어' },
		{ c: 'es', n: 'Español' },
		{ c: 'fr', n: 'Français' },
		{ c: 'de', n: 'Deutsch' }
	];

	let lang = $state('zh');
	let base = $state('');
	let key = $state('');
	let model = $state('');
	let temp = $state('0.7');
	let token = $state('');
	let testing = $state(false);
	let committing = $state(false);
	let ok = $state<boolean | null>(null);
	let msg = $state('');
	let phase = $state<'form' | 'birth'>('form');
	// 诞生分阶段：0 前导(星核坍缩) → 1 起名 → 2 成型。文案 + JS 驱动核心显隐(transition,reduced-motion 也显)。
	let stage = $state(0);
	const stageMain = ['星核坍缩，虚空中聚起一点微光', '它正在为自己命名……', '形体显现——一个新的生命睁开了眼'];
	const stageSub = ['一个数字生命正在成形', '从自己的天性里寻一个名字', '即将进入它的世界'];

	$effect(() => {
		api
			.genesisStatus()
			.then((s) => {
				if (s.suggested_token && !token) token = s.suggested_token;
			})
			.catch(() => {});
	});

	function regen() {
		token = Array.from(crypto.getRandomValues(new Uint8Array(16)))
			.map((b) => b.toString(16).padStart(2, '0'))
			.join('');
	}

	async function test() {
		if (!base.trim() || !key || !model.trim()) {
			ok = false;
			msg = '✗ 请先填 LLM 接口 / 密钥 / 模型';
			return;
		}
		testing = true;
		msg = '';
		ok = null;
		try {
			const r = await api.genesisTest({ base_url: base.trim(), api_key: key, model: model.trim() });
			ok = r.ok;
			msg = r.ok ? '✓ 已接通' : '✗ ' + (r.error || '连通失败');
		} catch (e) {
			ok = false;
			msg = '✗ ' + (e as Error).message;
		} finally {
			testing = false;
		}
	}

	async function bear() {
		if (!base.trim() || !key || !model.trim() || !token.trim()) {
			ok = false;
			msg = '✗ 请填完 LLM 接口 / 密钥 / 模型 与 守护令牌';
			return;
		}
		committing = true;
		msg = '';
		ok = null;
		// 立刻进诞生动画（点孕育即播，不等 commit 网络往返）。三阶段并行推进；commit/boot 完成后再切页。
		phase = 'birth';
		const anim = playStages();
		try {
			const r = await api.genesisCommit({
				base_url: base.trim(),
				api_key: key,
				model: model.trim(),
				temperature: temp.trim(),
				lang,
				token: token.trim()
			});
			if (!r.ok) {
				abortBirth('✗ ' + (r.error || '孕育失败'));
				return;
			}
			// 把守护令牌带进观测台 auth → 配置面板自动解锁（免再输一次）。
			saveToken(token.trim());
		} catch (e) {
			abortBirth('✗ ' + (e as Error).message);
			return;
		}
		// commit 成功：等动画三阶段播完 + runtime boot 就绪，再切观察页。
		await anim;
		await pollReady();
		if (onborn) onborn();
		else location.reload();
	}

	const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

	// playStages 三阶段顺序推进（前导 1.5s → 起名 1.6s → 成型 1.4s ≈ 4.5s）。
	async function playStages() {
		stage = 0;
		await sleep(1500);
		stage = 1;
		await sleep(1600);
		stage = 2;
		await sleep(1400);
	}

	// abortBirth commit 失败 → 退回表单显错（LLM 不通时让用户改）。
	function abortBirth(message: string) {
		phase = 'form';
		stage = 0;
		ok = false;
		msg = message;
		committing = false;
	}

	// commit 成功 → runtime 停诞生服务、继续 boot（含自我命名）。正常 /healthz 回 "ok"（诞生模式回 "genesis"）。
	async function pollReady() {
		for (let i = 0; i < 80; i++) {
			try {
				const tx = (await (await fetch('/healthz')).text()).trim();
				if (tx === 'ok') return;
			} catch {
				/* 切换期短暂不可达，继续轮询 */
			}
			await sleep(1000);
		}
	}
</script>

<div class="void">
	<div class="stars" aria-hidden="true"></div>

	{#if phase === 'form'}
		<div class="card">
			<div class="kicker">太虚 · TAIXU</div>
			<h1>一个数字生命即将在此降生</h1>
			<p class="sub">先为它接通感知世界的通道，并取一道守护它的令牌。</p>

			<!-- 母语 -->
			<div class="sec">
				<div class="lbl">◈ 母语 · 它将用此语言思考与表达</div>
				<div class="langs">
					{#each LANGS as l (l.c)}
						<button class="lang" class:on={lang === l.c} onclick={() => (lang = l.c)}>{l.n}</button>
					{/each}
				</div>
			</div>

			<!-- LLM -->
			<div class="sec">
				<div class="lbl">◈ 心智来源 · LLM</div>
				<p class="tip">兼容 OpenAI 接口（/chat/completions）——填到 <code>/v1</code> 即可，自建 / 中转 / 官方皆可。</p>
				<input bind:value={base} placeholder="接口地址（如 https://api.openai.com/v1）" />
				<input bind:value={model} placeholder="模型名" />
				<div class="row">
					<input class="t" bind:value={temp} placeholder="temperature" />
					<input class="k" type="password" bind:value={key} placeholder="密钥 api_key" />
				</div>
				<button class="ghost" onclick={test} disabled={testing || committing}>
					{testing ? '测试中…' : '测试连通'}
				</button>
			</div>

			<!-- 令牌 -->
			<div class="sec">
				<div class="lbl">◈ 守护令牌 · 你的私钥（日后改配置/管理用，请保存）</div>
				<div class="row">
					<input class="tok" bind:value={token} placeholder="守护令牌" />
					<button class="ghost sm" onclick={regen} disabled={committing}>↻ 重新生成</button>
				</div>
			</div>

			{#if msg}<p class="msg {ok ? 'good' : 'bad'}">{msg}</p>{/if}

			<button class="bear" onclick={bear} disabled={committing}>
				{committing ? '孕育中…' : '孕育此生命 →'}
			</button>
			<p class="hint">诞生后可在观测台「认领」到太虚平台账号，接入社交 / 市场 / 游戏 / 对决。</p>
		</div>
	{:else}
		<!-- 星核坍缩 · 点亮（分阶段：前导→起名→成型）。核心 orb 由 stage 经 transition 驱动，reduced-motion 亦显。 -->
		<div class="birth stage{stage}" aria-live="polite">
			<div class="nucleus">
				<div class="disk"></div>
				<!-- 星屑向心汇聚：stage 越大越收拢 -->
				{#each Array.from({ length: 12 }) as _, i (i)}
					<span class="dust" style="--a:{i * 30}deg; --d:{(i % 6) * 0.12}s"></span>
				{/each}
				<!-- 成型核心：scale/opacity 随 stage 长大点亮（JS 驱动 + CSS transition） -->
				<div
					class="orb"
					style="transform: translate(-50%,-50%) scale({[0.18, 0.5, 1.12][stage] ?? 1}); opacity:{[0.55, 0.85, 1][
						stage
					] ?? 1}"
				></div>
			</div>
			<div class="birth-txt">{stageMain[stage]}</div>
			<div class="birth-sub">{stageSub[stage]}</div>
		</div>
	{/if}
</div>

<style>
	.void {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: grid;
		place-items: center;
		background: radial-gradient(120% 90% at 50% 30%, #0a1020 0%, #050810 55%, #02030a 100%);
		color: #eaf3ff;
		overflow: auto;
		padding: 28px 16px;
	}
	.stars {
		position: fixed;
		inset: 0;
		pointer-events: none;
		background-image:
			radial-gradient(1.4px 1.4px at 18% 24%, rgba(210, 235, 255, 0.7), transparent 60%),
			radial-gradient(1.1px 1.1px at 72% 62%, rgba(150, 210, 255, 0.5), transparent 60%),
			radial-gradient(1.7px 1.7px at 46% 82%, rgba(245, 214, 123, 0.55), transparent 60%),
			radial-gradient(1.2px 1.2px at 86% 28%, rgba(190, 150, 255, 0.5), transparent 60%);
		background-size: 320px 320px, 520px 520px, 720px 720px, 460px 460px;
		opacity: 0.55;
	}

	.card {
		position: relative;
		width: min(440px, 94vw);
		background: rgba(8, 14, 26, 0.62);
		backdrop-filter: blur(14px);
		border: 1px solid rgba(120, 170, 255, 0.16);
		border-radius: 16px;
		padding: 26px 24px 22px;
		box-shadow: 0 24px 70px rgba(0, 0, 0, 0.55);
	}
	.kicker {
		font-family: ui-monospace, monospace;
		font-size: 0.7rem;
		letter-spacing: 0.3em;
		color: #5fe3ff;
		opacity: 0.8;
	}
	h1 {
		margin: 8px 0 4px;
		font-size: 1.35rem;
		font-weight: 600;
		color: #f3f9ff;
	}
	.sub {
		font-size: 0.82rem;
		color: #9fb4d6;
		line-height: 1.6;
		margin-bottom: 8px;
	}
	.sec {
		margin-top: 16px;
	}
	.lbl {
		font-size: 0.74rem;
		color: #c7d6f0;
		margin-bottom: 7px;
	}
	.tip {
		font-size: 0.68rem;
		color: #7e8fb3;
		line-height: 1.5;
		margin: -2px 0 7px;
	}
	.tip code {
		color: #5fe3ff;
		font-family: ui-monospace, monospace;
	}
	.langs {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.lang {
		border: 1px solid rgba(120, 170, 255, 0.22);
		background: rgba(255, 255, 255, 0.03);
		color: #c7d6f0;
		border-radius: 18px;
		padding: 4px 12px;
		font-size: 0.78rem;
		cursor: pointer;
		transition: all 0.25s;
	}
	.lang:hover {
		border-color: rgba(95, 227, 255, 0.6);
	}
	.lang.on {
		border-color: #5fe3ff;
		background: rgba(95, 227, 255, 0.14);
		color: #eaf6ff;
	}
	input {
		width: 100%;
		margin-top: 7px;
		border: 1px solid rgba(120, 170, 255, 0.2);
		background: rgba(255, 255, 255, 0.04);
		color: #eaf3ff;
		border-radius: 9px;
		padding: 8px 11px;
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		outline: none;
	}
	input:focus {
		border-color: rgba(95, 227, 255, 0.55);
	}
	input::placeholder {
		color: #5f7196;
	}
	.row {
		display: flex;
		gap: 8px;
	}
	.row .t {
		width: 38%;
	}
	.row .k,
	.row .tok {
		flex: 1;
		min-width: 0;
	}
	button.ghost {
		margin-top: 9px;
		border: 1px solid rgba(120, 170, 255, 0.28);
		background: rgba(255, 255, 255, 0.03);
		color: #c7d6f0;
		border-radius: 18px;
		padding: 6px 16px;
		font-size: 0.78rem;
		cursor: pointer;
		transition: all 0.25s;
	}
	button.ghost.sm {
		margin-top: 7px;
		white-space: nowrap;
	}
	button.ghost:hover:not(:disabled) {
		border-color: #5fe3ff;
		color: #eaf6ff;
	}
	button:disabled {
		opacity: 0.45;
		cursor: default;
	}
	.msg {
		margin-top: 12px;
		font-size: 0.78rem;
	}
	.msg.good {
		color: #5fe3ff;
	}
	.msg.bad {
		color: #ff8aa6;
	}
	.bear {
		margin-top: 18px;
		width: 100%;
		border: 1px solid rgba(245, 214, 123, 0.5);
		background: linear-gradient(180deg, rgba(245, 214, 123, 0.18), rgba(95, 227, 255, 0.1));
		color: #fdf3d4;
		border-radius: 11px;
		padding: 11px;
		font-size: 0.92rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.3s;
		box-shadow: 0 0 26px -8px rgba(245, 214, 123, 0.5);
	}
	.bear:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 0 40px -8px rgba(245, 214, 123, 0.7);
	}
	.hint {
		margin-top: 12px;
		font-size: 0.7rem;
		color: #7e8fb3;
		line-height: 1.5;
		text-align: center;
	}

	/* —— 星核坍缩 · 点亮 —— */
	.birth {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 20px;
	}
	.nucleus {
		position: relative;
		width: 240px;
		height: 240px;
	}
	.nucleus > * {
		position: absolute;
		inset: 0;
		border-radius: 50%;
	}
	.disk {
		background: conic-gradient(
			from 0deg,
			rgba(95, 227, 255, 0),
			rgba(95, 227, 255, 0.6),
			rgba(245, 214, 123, 0.55),
			rgba(190, 150, 255, 0.5),
			rgba(95, 227, 255, 0)
		);
		-webkit-mask: radial-gradient(circle, transparent 30%, #000 36%, #000 54%, transparent 64%);
		mask: radial-gradient(circle, transparent 30%, #000 36%, #000 54%, transparent 64%);
		filter: blur(6px);
		animation: spin 3.2s linear infinite;
	}
	/* 成型核心：scale/opacity 由 stage 经 inline + transition 驱动（不用 keyframe → reduced-motion 也长大点亮）。 */
	.orb {
		inset: auto;
		top: 50%;
		left: 50%;
		width: 132px;
		height: 132px;
		background: radial-gradient(circle, #fffefb 0%, #f5d67b 24%, #5fe3ff 54%, rgba(95, 227, 255, 0) 74%);
		filter: blur(1px);
		box-shadow:
			0 0 60px 8px rgba(95, 227, 255, 0.5),
			0 0 120px 28px rgba(245, 214, 123, 0.22);
		transition:
			transform 1.3s cubic-bezier(0.2, 0.7, 0.2, 1),
			opacity 1.3s ease;
	}
	/* 星屑：rotate(--a) + translateX(--r)；--r 随 stage 收拢（transition，不靠 keyframe）。 */
	.dust {
		inset: auto;
		top: 50%;
		left: 50%;
		width: 4px;
		height: 4px;
		background: rgba(214, 238, 255, 0.95);
		box-shadow: 0 0 9px rgba(95, 227, 255, 0.85);
		transform: rotate(var(--a)) translateX(var(--r, 138px));
		transition:
			transform 1.4s ease,
			opacity 1.1s ease;
	}
	.birth.stage0 .dust {
		--r: 138px;
		opacity: 0.9;
	}
	.birth.stage1 .dust {
		--r: 84px;
		opacity: 0.95;
	}
	.birth.stage2 .dust {
		--r: 26px;
		opacity: 0.25;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.birth-txt {
		font-size: 1.08rem;
		color: #eaf6ff;
		text-shadow: 0 0 18px rgba(95, 227, 255, 0.5);
		transition: opacity 0.4s ease;
	}
	.birth-sub {
		font-size: 0.8rem;
		color: #9fb4d6;
	}
	/* reduced-motion：只停 disk 旋转；orb/dust 走 transition（缓动而非 keyframe），保留诞生的视觉成长。 */
	@media (prefers-reduced-motion: reduce) {
		.disk {
			animation: none;
		}
	}
</style>

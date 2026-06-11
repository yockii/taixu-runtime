<script lang="ts">
	import { t, lang } from '$lib/i18n';
	import { skillVer } from '$lib/stores';
	import { unixToDate, apiPost, api } from '$lib/api';
	import { locked } from '$lib/auth';
	import TokenGate from './TokenGate.svelte';

	type Skill = {
		id: string;
		name: string;
		description?: string;
		status: string;
		pending_deps?: string;
		mastery: number;
		used_count: number;
		last_used_at?: number;
		created_at: number;
		lanes?: string;
		authored_from?: string;
	};

	let items = $state<Skill[]>([]);
	let autoApprove = $state(false);
	let proactiveIM = $state(false);
	let loadText = $state('');
	let busy = $state(false);
	let err = $state('');
	let busyIds = $state<Record<string, string>>({}); // skill id → 进行中动作文案

	async function load() {
		const r = await fetch('/api/skills?limit=50');
		if (r.ok) items = (await r.json()) ?? [];
		try {
			const cfg = await api.config(); // 带 token；未授权时无 toggle 字段（默认 false）
			autoApprove = !!cfg.skill_auto_approve_deps;
			proactiveIM = !!cfg.proactive_im;
		} catch {
			/* ignore */
		}
	}

	async function rescan() {
		busy = true;
		err = '';
		try {
			await apiPost('/api/skills/rescan');
			skillVer.update((n) => n + 1);
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			busy = false;
		}
	}

	async function toggleProactive() {
		const next = !proactiveIM;
		err = '';
		try {
			await apiPost('/api/config/proactive-im', { value: next });
			proactiveIM = next;
		} catch (e: any) {
			err = String(e?.message ?? e);
		}
	}

	$effect(() => {
		$skillVer;
		load();
	});
	$effect(() => {
		const ti = setInterval(load, 30000);
		return () => clearInterval(ti);
	});

	async function submitLoad() {
		const tx = loadText.trim();
		if (!tx) return;
		busy = true;
		err = '';
		try {
			await apiPost('/api/skills/load', { content: tx });
			loadText = '';
			skillVer.update((n) => n + 1);
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			busy = false;
		}
	}

	async function approve(id: string) {
		busyIds = { ...busyIds, [id]: $t('skill_status_installing') };
		err = '';
		try {
			await apiPost('/api/skills/approve', { id });
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			const { [id]: _, ...rest } = busyIds;
			busyIds = rest;
			skillVer.update((n) => n + 1);
		}
	}
	async function reject(id: string) {
		busyIds = { ...busyIds, [id]: '...' };
		err = '';
		try {
			await apiPost('/api/skills/reject', { id });
		} catch (e: any) {
			err = String(e?.message ?? e);
		} finally {
			const { [id]: _, ...rest } = busyIds;
			busyIds = rest;
			skillVer.update((n) => n + 1);
		}
	}
	async function toggleAuto() {
		const next = !autoApprove;
		err = '';
		try {
			await apiPost('/api/config/auto-approve-deps', { value: next });
			autoApprove = next;
		} catch (e: any) {
			err = String(e?.message ?? e);
		}
	}

	function parseDeps(s?: string): { runtime: string; package: string }[] {
		if (!s) return [];
		try {
			return JSON.parse(s) ?? [];
		} catch {
			return [];
		}
	}
	function statusColor(s: string): string {
		switch (s) {
			case 'ready':
				return 'text-glow';
			case 'pending_approval':
				return 'text-[#ffc97a]';
			case 'installing':
				return 'text-glowsoft';
			case 'failed':
				return 'text-[#ff7a96]';
			default:
				return 'text-dim';
		}
	}
	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');

	// 技能可能积累上百个 → 默认只显示计数，点击弹框看全部（R88-2）。
	let showModal = $state(false);
	const readyCount = $derived(items.filter((s) => s.status === 'ready').length);
	const selfCount = $derived(items.filter((s) => s.authored_from).length);
	const archivedCount = $derived(items.filter((s) => s.status === 'archived').length);
	const pendingCount = $derived(items.filter((s) => s.status === 'pending_approval').length);
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('skills_title')}</h2>

	<!-- 写控件区（装载 / 开关）：未授权时整体替换为「输入令牌」占位 -->
	<TokenGate>
		<!-- 装载入口 -->
		<form
		class="mb-3 space-y-2"
		onsubmit={(e) => {
			e.preventDefault();
			submitLoad();
		}}
	>
		<textarea
			class="h-16 w-full resize-none rounded-md border border-line bg-white/5 px-2 py-1 text-xs text-fog placeholder:text-dim outline-none focus:border-glow/50"
			placeholder={$t('skill_load_placeholder')}
			bind:value={loadText}
			disabled={busy}
		></textarea>
		<div class="flex items-center justify-between">
			{#if err}<span class="text-xs text-[#ff7a96]">{err}</span>{:else}<span class="text-xs text-dim">{$t('skill_dir_hint')}</span>{/if}
			<div class="flex gap-2">
				<button
					type="button"
					class="rounded-full border border-glow/40 bg-glow/10 px-3 py-1 text-xs font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
					disabled={busy}
					onclick={rescan}
				>
					{$t('skill_rescan_btn')}
				</button>
				<button
					type="submit"
					class="rounded-full border border-glow/40 bg-glow/10 px-3 py-1 text-xs font-medium text-glow transition hover:bg-glow/20 disabled:opacity-40"
					disabled={busy || !loadText.trim()}
				>
					{$t('skill_load_btn')}
				</button>
			</div>
		</div>
	</form>

	<!-- dangerous-skip toggle -->
	<label class="mb-2 flex items-start gap-2 rounded border border-[#ff7a96]/40 bg-[#ff7a96]/10 p-2 text-xs">
		<input type="checkbox" checked={autoApprove} onchange={toggleAuto} class="mt-0.5 accent-glow" />
		<span>
			<span class="font-semibold text-[#ff7a96]">{$t('skill_auto_approve')}</span>
			<span class="block text-[#ff7a96]/70">{$t('skill_auto_approve_warn')}</span>
		</span>
	</label>

	<!-- proactive IM toggle (B) -->
		<label class="mb-3 flex items-start gap-2 rounded border border-[#ffc97a]/40 bg-[#ffc97a]/10 p-2 text-xs">
			<input type="checkbox" checked={proactiveIM} onchange={toggleProactive} class="mt-0.5 accent-glow" />
			<span>
				<span class="font-semibold text-[#ffc97a]">{$t('proactive_im')}</span>
				<span class="block text-[#ffc97a]/70">{$t('proactive_im_warn')}</span>
			</span>
		</label>
	</TokenGate>

	<!-- 计数摘要 + 查看全部（避免上百技能铺满面板）-->
	{#if items.length === 0}
		<div class="tempty">{$t('empty_skill')}</div>
	{:else}
		<button
			onclick={() => (showModal = true)}
			class="flex w-full items-center justify-between rounded-lg border border-line bg-white/5 px-3 py-2 text-left text-xs transition hover:border-glow/40"
		>
			<span class="text-fog">
				{$t('skills_total')} <span class="font-semibold text-bright">{items.length}</span>
				<span class="ml-2 text-dim">
					ready {readyCount}
					{#if selfCount > 0}· {$t('skill_self')} {selfCount}{/if}
					{#if archivedCount > 0}· {$t('skill_status_archived')} {archivedCount}{/if}
				</span>
			</span>
			<span class="text-dim">
				{#if pendingCount > 0}<span class="mr-2 text-[#ffc97a]">{pendingCount} {$t('skill_status_pending_approval')}</span>{/if}
				{$t('view_all')} →
			</span>
		</button>
	{/if}
</div>

<!-- 弹框：完整技能列表 -->
{#if showModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
		onclick={() => (showModal = false)}
		onkeydown={(e) => e.key === 'Escape' && (showModal = false)}
		role="button"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
		<div
			class="card max-h-[80vh] w-full max-w-2xl overflow-hidden"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-sm font-semibold text-fog">{$t('skills_title')} · {items.length}</h2>
				<button onclick={() => (showModal = false)} class="text-dim hover:text-bright">✕</button>
			</div>
			<div class="max-h-[68vh] space-y-2 overflow-y-auto text-xs">
				{#each items as s (s.id)}
					<div class="border-b border-line py-1">
					<div class="flex items-baseline gap-2">
						<span class="font-medium text-bright">{s.name}</span>
						{#if s.authored_from}
							<span class="shrink-0 rounded bg-violet/20 px-1 text-violet" title={s.authored_from}>{$t('skill_self')}</span>
						{/if}
						<span class="{statusColor(s.status)} shrink-0">{$t('skill_status_' + s.status)}</span>
						<span class="flex-1"></span>
						<span class="text-dim">m{s.mastery?.toFixed?.(2) ?? '0.00'} · {$t('skill_used')} {s.used_count}</span>
					</div>
					{#if s.description}
						<div class="mt-0.5 truncate text-fog">{s.description}</div>
					{/if}
					{#if s.status === 'pending_approval'}
						<div class="mt-1 rounded bg-white/5 p-1.5">
							<div class="mb-1 text-[#ffc97a]">{$t('skill_pending_deps')}</div>
							{#each parseDeps(s.pending_deps) as d}
								<div class="font-mono text-fog">
									· {d.runtime}: {d.package}
									<a
										class="ml-1 text-glowsoft hover:underline"
										href={d.runtime === 'python'
											? `https://pypi.org/project/${d.package.split(/[<>=~[]/)[0]}/`
											: `https://www.npmjs.com/package/${d.package.split(/[<>=~]/)[0]}`}
										target="_blank"
										rel="noopener">↗</a
									>
								</div>
							{/each}
							<div class="mt-1.5 flex items-center gap-2">
								{#if busyIds[s.id]}
									<span class="italic text-glowsoft">{busyIds[s.id]}</span>
								{:else if $locked}
									<span class="text-dim">🔒 {$t('locked_hint')}</span>
								{:else}
									<button
										class="rounded-full border border-glow/40 bg-glow/10 px-2 py-0.5 text-glow transition hover:bg-glow/20"
										onclick={() => approve(s.id)}>{$t('skill_approve')}</button
									>
									<button
										class="rounded-full border border-[#ff7a96]/40 bg-[#ff7a96]/10 px-2 py-0.5 text-[#ff7a96] transition hover:bg-[#ff7a96]/20"
										onclick={() => reject(s.id)}>{$t('skill_reject')}</button
									>
								{/if}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
		</div>
	</div>
{/if}

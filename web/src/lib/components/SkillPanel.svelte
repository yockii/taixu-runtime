<script lang="ts">
	import { t, lang } from '$lib/i18n';
	import { skillVer } from '$lib/stores';
	import { unixToDate } from '$lib/api';

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
	};

	let items = $state<Skill[]>([]);
	let autoApprove = $state(false);
	let loadText = $state('');
	let busy = $state(false);
	let err = $state('');
	let busyIds = $state<Record<string, string>>({}); // skill id → 进行中动作文案

	async function load() {
		const r = await fetch('/api/skills?limit=50');
		if (r.ok) items = (await r.json()) ?? [];
		const c = await fetch('/api/config');
		if (c.ok) {
			const cfg = await c.json();
			autoApprove = !!cfg.skill_auto_approve_deps;
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
			const r = await fetch('/api/skills/load', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ content: tx })
			});
			if (!r.ok) throw new Error(await r.text());
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
		try {
			const r = await fetch('/api/skills/approve', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ id })
			});
			if (!r.ok) err = await r.text();
		} finally {
			const { [id]: _, ...rest } = busyIds;
			busyIds = rest;
			skillVer.update((n) => n + 1);
		}
	}
	async function reject(id: string) {
		busyIds = { ...busyIds, [id]: '...' };
		try {
			await fetch('/api/skills/reject', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ id })
			});
		} finally {
			const { [id]: _, ...rest } = busyIds;
			busyIds = rest;
			skillVer.update((n) => n + 1);
		}
	}
	async function toggleAuto() {
		const next = !autoApprove;
		await fetch('/api/config/auto-approve-deps', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ value: next })
		});
		autoApprove = next;
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
				return 'text-emerald-400';
			case 'pending_approval':
				return 'text-amber-400';
			case 'installing':
				return 'text-cyan-400';
			case 'failed':
				return 'text-rose-400';
			default:
				return 'text-zinc-500';
		}
	}
	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-zinc-400">{$t('skills_title')}</h2>

	<!-- 装载入口 -->
	<form
		class="mb-3 space-y-2"
		onsubmit={(e) => {
			e.preventDefault();
			submitLoad();
		}}
	>
		<textarea
			class="h-16 w-full resize-none rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-xs focus:border-zinc-500 focus:outline-none"
			placeholder={$t('skill_load_placeholder')}
			bind:value={loadText}
			disabled={busy}
		></textarea>
		<div class="flex items-center justify-between">
			{#if err}<span class="text-xs text-rose-400">{err}</span>{:else}<span></span>{/if}
			<button
				type="submit"
				class="rounded bg-emerald-600 px-3 py-1 text-xs font-medium hover:bg-emerald-500 disabled:opacity-50"
				disabled={busy || !loadText.trim()}
			>
				{$t('skill_load_btn')}
			</button>
		</div>
	</form>

	<!-- dangerous-skip toggle -->
	<label class="mb-3 flex items-start gap-2 rounded border border-rose-800/50 bg-rose-950/20 p-2 text-xs">
		<input type="checkbox" checked={autoApprove} onchange={toggleAuto} class="mt-0.5" />
		<span>
			<span class="font-semibold text-rose-300">{$t('skill_auto_approve')}</span>
			<span class="block text-rose-400/70">{$t('skill_auto_approve_warn')}</span>
		</span>
	</label>

	{#if items.length === 0}
		<div class="text-sm text-zinc-500">{$t('empty_skill')}</div>
	{:else}
		<div class="max-h-80 space-y-2 overflow-y-auto text-xs">
			{#each items as s (s.id)}
				<div class="border-b border-zinc-800 py-1">
					<div class="flex items-baseline gap-2">
						<span class="font-medium text-zinc-200">{s.name}</span>
						<span class="{statusColor(s.status)} shrink-0">{$t('skill_status_' + s.status)}</span>
						<span class="flex-1"></span>
						<span class="text-zinc-500">{$t('skill_used')} {s.used_count}</span>
					</div>
					{#if s.description}
						<div class="mt-0.5 truncate text-zinc-400">{s.description}</div>
					{/if}
					{#if s.status === 'pending_approval'}
						<div class="mt-1 rounded bg-zinc-900/60 p-1.5">
							<div class="mb-1 text-amber-300">{$t('skill_pending_deps')}</div>
							{#each parseDeps(s.pending_deps) as d}
								<div class="font-mono text-zinc-300">
									· {d.runtime}: {d.package}
									<a
										class="ml-1 text-cyan-500 hover:underline"
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
									<span class="italic text-cyan-400">{busyIds[s.id]}</span>
								{:else}
									<button
										class="rounded bg-emerald-600 px-2 py-0.5 hover:bg-emerald-500"
										onclick={() => approve(s.id)}>{$t('skill_approve')}</button
									>
									<button
										class="rounded bg-zinc-700 px-2 py-0.5 hover:bg-zinc-600"
										onclick={() => reject(s.id)}>{$t('skill_reject')}</button
									>
								{/if}
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

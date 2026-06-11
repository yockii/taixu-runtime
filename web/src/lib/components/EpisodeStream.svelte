<script lang="ts">
	import { api, type Episode, unixToDate } from '$lib/api';
	import { t, lang } from '$lib/i18n';
	import { episodeVer } from '$lib/stores';

	let q = $state('');
	let episodes = $state<Episode[]>([]);
	let loading = $state(false);
	let timer: ReturnType<typeof setTimeout> = null!;

	async function load() {
		loading = true;
		try {
			const out = await api.episodes(q, 30, 0);
			episodes = out ?? [];
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		$episodeVer;
		load();
		return () => clearTimeout(timer); // 销毁时清 debounce 定时器
	});

	function debouncedSearch() {
		clearTimeout(timer);
		timer = setTimeout(load, 300);
	}

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
</script>

<div class="card">
	<div class="mb-3 flex items-center justify-between">
		<h2 class="text-sm font-semibold text-fog">{$t('episodes_title')}</h2>
		<input
			type="search"
			class="rounded-md border border-line bg-white/5 px-2 py-1 text-xs outline-none focus:border-glow/50"
			placeholder={$t('search_placeholder')}
			bind:value={q}
			oninput={debouncedSearch}
		/>
	</div>
	{#if loading && episodes.length === 0}
		<div class="text-sm text-dim">{$t('loading')}</div>
	{:else if episodes.length === 0}
		<div class="tempty">{$t('empty_episode')}</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto">
			{#each episodes as ep (ep.id)}
				<div class="border-l-2 border-line pl-3">
					<div class="flex items-baseline justify-between text-xs">
						<span class="font-mono text-dim">#{ep.id}</span>
						<span class="text-dim">{unixToDate(ep.started_at, locale)} · {ep.ended_at - ep.started_at}s</span>
					</div>
					<div class="mt-1 text-sm">{ep.summary}</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

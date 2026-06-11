<script lang="ts">
	// 实况流：所有 SSE 事件归一的时间线。打开面板即懂「它此刻在干什么」。
	// 条目由 +page.svelte 推入 feed store（环形 200 条），这里只渲染。
	import { feed } from '$lib/stores';
	import { t, lang } from '$lib/i18n';

	const locale = $derived($lang === 'zh' ? 'zh-CN' : 'en-US');
	const hhmmss = (unix: number) =>
		new Date(unix * 1000).toLocaleTimeString(locale, { hour12: false });
</script>

<div class="card">
	<h2 class="mb-3 text-sm font-semibold text-fog">{$t('live_title')}</h2>
	{#if $feed.length === 0}
		<p class="tempty">{$t('live_empty')}</p>
	{:else}
		<ul class="max-h-[60vh] space-y-1.5 overflow-y-auto pr-1">
			{#each $feed as ev (ev.id)}
				<li class="fev {ev.tone} rounded-r-md py-1.5 pr-2 pl-3 text-xs">
					<div class="flex items-baseline gap-2">
						<span class="shrink-0 font-semibold" class:text-glow={ev.tone === 'glow'} class:text-glowsoft={ev.tone === 'cool'} class:text-violet={ev.tone === 'violet'} class:text-warm={ev.tone === 'warm'} class:text-dim={ev.tone === 'dim'}>{$t(ev.title)}</span>
						<span class="ml-auto shrink-0 text-[10px] text-dim tabular-nums">{hhmmss(ev.at)}</span>
					</div>
					{#if ev.text}
						<p class="mt-0.5 leading-relaxed break-words whitespace-pre-wrap text-fog">{ev.text}</p>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>

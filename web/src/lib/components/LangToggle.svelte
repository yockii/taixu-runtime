<script lang="ts">
	import { lang, toggleLang } from '$lib/i18n';
</script>

<button
	type="button"
	class="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-400 transition hover:border-zinc-500 hover:text-zinc-200"
	onclick={toggleLang}
	title={$lang === 'zh' ? 'Switch to English' : '切换到中文'}
>
	{$lang === 'zh' ? 'EN' : '中'}
</button>

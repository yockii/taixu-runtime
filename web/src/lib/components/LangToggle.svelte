<script lang="ts">
	import { lang, toggleLang } from '$lib/i18n';
</script>

<button
	type="button"
	class="rounded-full border border-glow/40 bg-glow/10 px-2 py-0.5 text-xs text-glow transition hover:bg-glow/20"
	onclick={toggleLang}
	title={$lang === 'zh' ? 'Switch to English' : '切换到中文'}
>
	{$lang === 'zh' ? 'EN' : '中'}
</button>

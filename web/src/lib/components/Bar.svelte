<script lang="ts">
	let { label, value, color = 'bg-emerald-500' }: { label: string; value: number; color?: string } = $props();
	const pct = $derived(Math.round(value * 100));
</script>

<div>
	<div class="mb-1 flex justify-between text-xs">
		<span class="text-zinc-400">{label}</span>
		<span class="text-zinc-300 tabular-nums">{pct}%</span>
	</div>
	<div class="bar">
		<div class="bar-fill {color}" style="width: {pct}%"></div>
	</div>
</div>

<script lang="ts">
	import { cn } from '$lib/utils.js';

	interface Props {
		value: string;
		readonly?: boolean;
		error?: string | null;
		class?: string;
		onchange?: (value: string) => void;
	}

	let {
		value = $bindable(''),
		readonly = false,
		error = null,
		class: className,
		onchange
	}: Props = $props();

	function handleInput(e: Event) {
		const target = e.target as HTMLTextAreaElement;
		value = target.value;
		onchange?.(value);
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Tab') {
			e.preventDefault();
			const target = e.target as HTMLTextAreaElement;
			const start = target.selectionStart;
			const end = target.selectionEnd;
			value = value.substring(0, start) + '  ' + value.substring(end);
			requestAnimationFrame(() => {
				target.selectionStart = target.selectionEnd = start + 2;
			});
		}
	}
</script>

<div class="relative">
	<textarea
		class={cn(
			'w-full min-h-[400px] font-mono text-sm leading-relaxed',
			'bg-muted/50 rounded-lg border p-4',
			'focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent',
			'resize-y',
			readonly && 'cursor-default bg-muted/30',
			error && 'border-destructive focus:ring-destructive',
			className
		)}
		{readonly}
		disabled={readonly}
		spellcheck="false"
		autocomplete="off"
		{value}
		oninput={handleInput}
		onkeydown={handleKeyDown}
	></textarea>
	{#if error}
		<p class="mt-2 text-sm text-destructive">{error}</p>
	{/if}
</div>

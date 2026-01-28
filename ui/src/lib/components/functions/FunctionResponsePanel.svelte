<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import Play from '@lucide/svelte/icons/play';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import Copy from '@lucide/svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import type { FunctionInvokeResponse } from '$lib/api/client';

	interface Props {
		response: FunctionInvokeResponse | null;
		isLoading: boolean;
		autofocus?: boolean;
	}

	let { response, isLoading, autofocus = false }: Props = $props();

	let responseContainer = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (autofocus && response && !isLoading && responseContainer) {
			responseContainer.focus();
		}
	});

	function formatJSON(data: unknown): string {
		try {
			return JSON.stringify(data, null, 2);
		} catch {
			return String(data);
		}
	}

	function getDisplayContent(): string {
		if (!response) return '';
		if (response.success) {
			return formatJSON(response.output);
		}
		return response.error || 'Unknown error';
	}

	async function copyToClipboard() {
		if (!response) return;
		const content = getDisplayContent();
		try {
			await navigator.clipboard.writeText(content);
			toast.success('Copied to clipboard');
		} catch {
			toast.error('Failed to copy');
		}
	}

	function syntaxHighlight(json: string): string {
		return json
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(
				/("(?:\\.|[^"\\])*")/g,
				'<span style="color:#4ade80">$1</span>'
			)
			.replace(/\b(true|false|null)\b/g, '<span style="color:#c084fc">$1</span>')
			.replace(/\b(\d+(?:\.\d+)?)\b/g, '<span style="color:#fbbf24">$1</span>');
	}
</script>

<div
	class="flex h-full flex-col"
	bind:this={responseContainer}
	tabindex="-1"
	role="region"
	aria-label="Function response"
	aria-live="polite"
>
	<div class="flex items-center justify-between border-b px-4 py-3">
		<div class="flex items-center gap-3">
			<h3 class="text-sm font-medium">Response</h3>
			{#if response && !isLoading}
				<Badge variant={response.success ? 'default' : 'destructive'}>
					{response.success ? 'Success' : 'Error'}
				</Badge>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if response?.duration_ms !== undefined && !isLoading}
				<span class="text-muted-foreground text-xs">{response.duration_ms}ms</span>
			{/if}
			{#if response && !isLoading}
				<Button variant="ghost" size="icon" class="size-7" onclick={copyToClipboard} aria-label="Copy response to clipboard">
					<Copy class="size-4" />
				</Button>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto p-4">
		{#if isLoading}
			<div class="space-y-2">
				<Skeleton class="h-4 w-3/4" />
				<Skeleton class="h-4 w-1/2" />
				<Skeleton class="h-4 w-2/3" />
				<Skeleton class="h-4 w-1/3" />
				<Skeleton class="h-4 w-3/4" />
			</div>
		{:else if response}
			{@const content = getDisplayContent()}
			<pre
				class="text-muted-foreground font-mono text-sm whitespace-pre-wrap break-all"
				>{@html syntaxHighlight(content)}</pre>
		{:else}
			<div class="text-muted-foreground flex h-full flex-col items-center justify-center gap-2">
				<Play class="size-8 opacity-50" />
				<p class="text-sm">Execute a function to see the response</p>
			</div>
		{/if}
	</div>
</div>

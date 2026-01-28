<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import CheckCircle from '@lucide/svelte/icons/check-circle';
	import XCircle from '@lucide/svelte/icons/x-circle';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import History from '@lucide/svelte/icons/history';
	import type { FunctionInvokeResponse } from '$lib/api/client';

	export interface HistoryItem {
		id: string;
		input: unknown;
		response: FunctionInvokeResponse;
		timestamp: Date;
	}

	interface Props {
		history: HistoryItem[];
		onSelect: (item: HistoryItem) => void;
		onClear: () => void;
	}

	let { history, onSelect, onClear }: Props = $props();

	function truncateInput(input: unknown): string {
		try {
			const str = JSON.stringify(input);
			if (str.length <= 50) return str;
			return str.slice(0, 47) + '...';
		} catch {
			return String(input).slice(0, 50);
		}
	}

	function formatDuration(ms?: number): string {
		if (ms === undefined) return '';
		return `${ms}ms`;
	}

	function formatRelativeTime(date: Date): string {
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const seconds = Math.floor(diff / 1000);
		const minutes = Math.floor(seconds / 60);
		const hours = Math.floor(minutes / 60);
		const days = Math.floor(hours / 24);

		if (seconds < 5) return 'just now';
		if (seconds < 60) return `${seconds}s ago`;
		if (minutes < 60) return `${minutes}m ago`;
		if (hours < 24) return `${hours}h ago`;
		if (days < 7) return `${days}d ago`;
		return date.toLocaleDateString();
	}
</script>

<div class="flex h-full flex-col">
	<div class="flex items-center justify-between border-b px-4 py-3">
		<div class="flex items-center gap-3">
			<h3 class="text-sm font-medium">History</h3>
			<Badge variant="secondary">{history.length}</Badge>
		</div>
		{#if history.length > 0}
			<Button variant="ghost" size="sm" class="h-7 gap-1.5" onclick={onClear}>
				<Trash2 class="size-3.5" />
				Clear
			</Button>
		{/if}
	</div>

	<div class="flex-1 overflow-auto">
		{#if history.length === 0}
			<div class="text-muted-foreground flex h-full flex-col items-center justify-center gap-2 p-8">
				<History class="size-8 opacity-50" />
				<p class="text-sm">No execution history yet</p>
				<p class="text-xs text-muted-foreground/70">Run a function to see history</p>
			</div>
		{:else}
			<div class="divide-y">
				{#each history as item (item.id)}
					<button
						class="w-full px-4 py-3 text-left transition-colors hover:bg-muted/50 focus:bg-muted/50 focus:outline-none"
						onclick={() => onSelect(item)}
					>
						<div class="flex items-start gap-3">
							<div class="mt-0.5 shrink-0">
								{#if item.response.success}
									<CheckCircle class="size-4 text-green-500" />
								{:else}
									<XCircle class="size-4 text-red-500" />
								{/if}
							</div>
							<div class="flex-1 min-w-0">
								<p class="font-mono text-xs text-muted-foreground truncate">
									{truncateInput(item.input)}
								</p>
								<div class="mt-1 flex items-center gap-2 text-xs text-muted-foreground/70">
									{#if item.response.duration_ms !== undefined}
										<span>{formatDuration(item.response.duration_ms)}</span>
										<span>·</span>
									{/if}
									<span>{formatRelativeTime(item.timestamp)}</span>
								</div>
							</div>
						</div>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</div>

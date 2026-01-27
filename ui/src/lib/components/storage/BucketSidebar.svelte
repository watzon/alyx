<script lang="ts">
	import type { Bucket } from '$lib/api/client';
	import { Badge } from '$ui/badge';
	import * as Card from '$ui/card';
	import DatabaseIcon from '@lucide/svelte/icons/database';

	interface Props {
		buckets: Bucket[];
		selectedBucket: string | null;
		onSelect: (name: string) => void;
	}

	let { buckets, selectedBucket, onSelect }: Props = $props();
</script>

<div class="space-y-2">
	{#if buckets.length === 0}
		<Card.Root>
			<Card.Content class="py-6 text-center">
				<DatabaseIcon class="mx-auto h-8 w-8 text-muted-foreground/50 mb-3" />
				<p class="text-sm text-muted-foreground">No buckets defined</p>
			</Card.Content>
		</Card.Root>
	{:else}
		{#each buckets as bucket}
			<button
				class="w-full text-left p-3 rounded-lg hover:bg-accent transition-colors {selectedBucket === bucket.name ? 'bg-accent' : ''}"
				onclick={() => onSelect(bucket.name)}
			>
				<div class="flex items-center justify-between gap-2">
					<span class="font-medium truncate">{bucket.name}</span>
					<Badge variant="secondary">{bucket.backend}</Badge>
				</div>
			</button>
		{/each}
	{/if}
</div>

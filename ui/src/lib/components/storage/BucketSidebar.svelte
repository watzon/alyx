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

{#if buckets.length === 0}
	<Card.Root>
		<Card.Content class="py-6 text-center">
			<DatabaseIcon class="mx-auto h-8 w-8 text-muted-foreground/50 mb-3" />
			<p class="text-sm text-muted-foreground">No buckets defined</p>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
		{#each buckets as bucket}
			<button
				class="text-left p-4 rounded-lg border transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-border/20 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30 {selectedBucket === bucket.name ? '!bg-muted/30 !backdrop-blur-xl !border-border/40' : ''}"
				onclick={() => onSelect(bucket.name)}
			>
				<div class="flex items-center justify-between gap-2">
					<span class="font-medium truncate">{bucket.name}</span>
					<Badge variant="secondary">{bucket.backend}</Badge>
				</div>
			</button>
		{/each}
	</div>
{/if}

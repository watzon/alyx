<script lang="ts">
	import type { Collection } from '$lib/api/client';
	import { Badge } from '$ui/badge';
	import * as Card from '$ui/card';
	import DatabaseIcon from '@lucide/svelte/icons/database';

	interface Props {
		collections: Collection[];
		selectedCollection: string | null;
		onSelect: (name: string) => void;
	}

	let { collections, selectedCollection, onSelect }: Props = $props();
</script>

{#if collections.length === 0}
	<Card.Root>
		<Card.Content class="py-10 text-center">
			<DatabaseIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
			<h3 class="mt-3 text-sm font-medium">No collections defined</h3>
			<p class="mt-1 text-sm text-muted-foreground">
				Add collections to your schema to get started
			</p>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
		{#each collections as collection}
			<button
				class="text-left p-4 rounded-lg border transition-colors hover:bg-accent {selectedCollection === collection.name ? 'bg-accent border-accent' : 'bg-card'}"
				onclick={() => onSelect(collection.name)}
			>
				<div class="flex items-center justify-between gap-2">
					<span class="font-medium truncate">{collection.name}</span>
					<Badge variant="secondary">{collection.fields.length} fields</Badge>
				</div>
			</button>
		{/each}
	</div>
{/if}

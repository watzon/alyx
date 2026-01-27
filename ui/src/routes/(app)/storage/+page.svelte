<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, files, type Schema, type Bucket } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';

	let selectedBucket = $state<string | null>(null);
	let viewMode = $state<'table' | 'grid'>('table');
	let search = $state('');
	let mimeType = $state('');

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const filesQuery = createQuery(() => ({
		queryKey: ['files', selectedBucket, search, mimeType],
		queryFn: async () => {
			if (!selectedBucket) return null;
			const result = await files.list(selectedBucket, { search, mime_type: mimeType });
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: !!selectedBucket
	}));
</script>

<div class="max-w-screen-2xl mx-auto h-[calc(100vh-4rem)]">
	<div class="grid grid-cols-[280px_1fr] gap-6 h-full">
		<!-- Left Sidebar: Buckets -->
		<div class="border-r pr-6">
			<h2 class="text-lg font-semibold mb-4">Buckets</h2>
			{#if schemaQuery.isPending}
				<Skeleton class="h-20 w-full" />
			{:else if schemaQuery.data?.buckets}
				{#each schemaQuery.data.buckets as bucket}
					<button
						class="w-full text-left p-3 rounded-lg hover:bg-accent {selectedBucket === bucket.name
							? 'bg-accent'
							: ''}"
						onclick={() => (selectedBucket = bucket.name)}
					>
						{bucket.name}
					</button>
				{/each}
			{:else}
				<p class="text-sm text-muted-foreground">No buckets defined</p>
			{/if}
		</div>

		<!-- Right Content: File Browser -->
		<div>
			{#if !selectedBucket}
				<Card.Root class="flex items-center justify-center h-full">
					<Card.Content class="text-center">
						<p class="text-muted-foreground">Select a bucket to view files</p>
					</Card.Content>
				</Card.Root>
			{:else}
				<div>
					<h2 class="text-lg font-semibold mb-4">{selectedBucket}</h2>
					<!-- Placeholder for FileBrowser component -->
					<p class="text-sm text-muted-foreground">File browser will go here</p>
				</div>
			{/if}
		</div>
	</div>
</div>

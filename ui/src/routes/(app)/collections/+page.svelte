<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type Schema } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import DatabaseIcon from 'lucide-svelte/icons/database';
	import ChevronRightIcon from 'lucide-svelte/icons/chevron-right';

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Collections</h1>
		<p class="text-sm text-muted-foreground">Browse and manage your data collections</p>
	</div>

	{#if schemaQuery.isPending}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each Array(6) as _}
				<Card.Root>
					<Card.Header>
						<Skeleton class="h-4 w-28" />
					</Card.Header>
					<Card.Content>
						<Skeleton class="h-4 w-20" />
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{:else if schemaQuery.isError}
		<Card.Root class="border-destructive/50">
			<Card.Content class="py-4">
				<p class="text-sm text-destructive">Failed to load collections</p>
			</Card.Content>
		</Card.Root>
	{:else if schemaQuery.data?.collections?.length === 0}
		<Card.Root>
			<Card.Content class="py-10 text-center">
				<DatabaseIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
				<h3 class="mt-3 text-sm font-medium">No collections</h3>
				<p class="mt-1 text-sm text-muted-foreground">Define collections in your schema.yaml to get started</p>
			</Card.Content>
		</Card.Root>
	{:else}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each schemaQuery.data?.collections ?? [] as collection}
				<a
					href="/_admin/collections/{collection.name}"
					class="group block"
				>
					<Card.Root class="transition-colors hover:border-foreground/20 hover:bg-card/80">
						<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
							<Card.Title class="text-sm font-medium">{collection.name}</Card.Title>
							<ChevronRightIcon class="h-4 w-4 text-muted-foreground/50 transition-transform group-hover:translate-x-0.5 group-hover:text-muted-foreground" />
						</Card.Header>
						<Card.Content>
							<div class="flex items-center gap-1.5">
								<Badge variant="secondary">{collection.fields.length} fields</Badge>
								{#if collection.indexes?.length}
									<Badge variant="outline">{collection.indexes.length} indexes</Badge>
								{/if}
							</div>
						</Card.Content>
					</Card.Root>
				</a>
			{/each}
		</div>
	{/if}
</div>

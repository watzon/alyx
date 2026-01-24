<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type Schema } from '$lib/api/client';
	import * as Card from '$ui/card';
	import * as Tabs from '$ui/tabs';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import KeyIcon from 'lucide-svelte/icons/key';
	import LinkIcon from 'lucide-svelte/icons/link';
	import HashIcon from 'lucide-svelte/icons/hash';

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	function getFieldTypeColor(type: string): string {
		switch (type) {
			case 'uuid':
				return 'bg-purple-500/10 text-purple-500';
			case 'string':
			case 'text':
				return 'bg-green-500/10 text-green-500';
			case 'int':
			case 'float':
				return 'bg-blue-500/10 text-blue-500';
			case 'bool':
				return 'bg-amber-500/10 text-amber-500';
			case 'timestamp':
				return 'bg-cyan-500/10 text-cyan-500';
			case 'json':
				return 'bg-pink-500/10 text-pink-500';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Schema</h1>
		<p class="text-sm text-muted-foreground">View your database schema definitions</p>
	</div>

	{#if schemaQuery.isPending}
		<Card.Root>
			<Card.Content class="py-6">
				<Skeleton class="h-48 w-full" />
			</Card.Content>
		</Card.Root>
	{:else if schemaQuery.isError}
		<Card.Root class="border-destructive">
			<Card.Content class="py-6">
				<p class="text-destructive">Failed to load schema</p>
			</Card.Content>
		</Card.Root>
	{:else if schemaQuery.data}
		<Tabs.Root value={schemaQuery.data.collections?.[0]?.name}>
			<Tabs.List class="w-full justify-start overflow-x-auto">
				{#each schemaQuery.data.collections ?? [] as collection}
					<Tabs.Trigger value={collection.name}>{collection.name}</Tabs.Trigger>
				{/each}
			</Tabs.List>

			{#each schemaQuery.data.collections ?? [] as collection}
				<Tabs.Content value={collection.name} class="space-y-4">
					<Card.Root>
						<Card.Header>
							<Card.Title>Fields</Card.Title>
						</Card.Header>
						<Card.Content>
							<div class="space-y-2">
								{#each collection.fields as field}
									<div
										class="flex items-center justify-between rounded-md border border-border p-3"
									>
										<div class="flex items-center gap-3">
											<span class="font-mono text-sm">{field.name}</span>
											<Badge variant="outline" class={getFieldTypeColor(field.type)}>
												{field.type}
											</Badge>
										</div>
										<div class="flex items-center gap-2">
											{#if field.primary}
												<Badge variant="secondary" class="gap-1">
													<KeyIcon class="h-3 w-3" />
													Primary
												</Badge>
											{/if}
											{#if field.references}
												<Badge variant="secondary" class="gap-1">
													<LinkIcon class="h-3 w-3" />
													{field.references}
												</Badge>
											{/if}
											{#if field.unique}
												<Badge variant="outline">Unique</Badge>
											{/if}
											{#if field.nullable}
												<Badge variant="outline">Nullable</Badge>
											{/if}
											{#if field.index}
												<Badge variant="outline" class="gap-1">
													<HashIcon class="h-3 w-3" />
													Indexed
												</Badge>
											{/if}
										</div>
									</div>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>

					{#if collection.indexes?.length}
						<Card.Root>
							<Card.Header>
								<Card.Title>Indexes</Card.Title>
							</Card.Header>
							<Card.Content>
								<div class="space-y-2">
									{#each collection.indexes as index}
										<div class="flex items-center justify-between rounded-md border border-border p-3">
											<span class="font-mono text-sm">{index.name}</span>
											<div class="flex items-center gap-2">
												<span class="text-sm text-muted-foreground">
													{index.fields.join(', ')}
												</span>
												{#if index.unique}
													<Badge variant="outline">Unique</Badge>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</Card.Content>
						</Card.Root>
					{/if}

					{#if collection.rules}
						<Card.Root>
							<Card.Header>
								<Card.Title>Access Rules</Card.Title>
							</Card.Header>
							<Card.Content>
								<div class="space-y-2">
									{#each Object.entries(collection.rules) as [operation, rule]}
										<div class="rounded-md border border-border p-3">
											<div class="flex items-center gap-2 mb-2">
												<Badge variant="secondary">{operation}</Badge>
											</div>
											<code class="text-sm font-mono text-muted-foreground">{rule}</code>
										</div>
									{/each}
								</div>
							</Card.Content>
						</Card.Root>
					{/if}
				</Tabs.Content>
			{/each}
		</Tabs.Root>
	{/if}
</div>

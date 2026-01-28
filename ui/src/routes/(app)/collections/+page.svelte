<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { collections, admin, type Collection } from '$lib/api/client';
	import { configStore } from '$lib/stores/config.svelte';
	import { CollectionDrawer, CollectionTable, CollectionSelector } from '$lib/components/collections';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Skeleton } from '$ui/skeleton';
	import * as Card from '$ui/card';
	import * as Tooltip from '$ui/tooltip';
	import RefreshCwIcon from 'lucide-svelte/icons/refresh-cw';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import SearchIcon from 'lucide-svelte/icons/search';
	import ChevronLeftIcon from 'lucide-svelte/icons/chevron-left';
	import ChevronRightIcon from 'lucide-svelte/icons/chevron-right';
	import ChevronsLeftIcon from 'lucide-svelte/icons/chevrons-left';
	import ChevronsRightIcon from 'lucide-svelte/icons/chevrons-right';
	import BookOpenIcon from 'lucide-svelte/icons/book-open';
	import DatabaseIcon from '@lucide/svelte/icons/database';

	let selectedCollection = $state<string | null>(null);
	let pageIndex = $state(1);
	let pageSize = $state(50);
	let search = $state('');
	let drawerOpen = $state(false);
	let editDocument = $state<Record<string, any> | null>(null);

	const schemaQuery = createQuery(() => ({
		queryKey: ['schema'],
		queryFn: async () => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
	}));

	const selectedCollectionSchema = $derived.by(() => {
		if (!schemaQuery.data || !selectedCollection) return null;
		return schemaQuery.data.collections?.find((c) => c.name === selectedCollection) || null;
	});

	const collectionDocsUrl = $derived(selectedCollection ? configStore.getCollectionDocsUrl(selectedCollection) : null);

	const docsQuery = createQuery(() => ({
		queryKey: ['collections', selectedCollection, 'documents', pageIndex, pageSize, search],
		queryFn: async () => {
			if (!selectedCollection) throw new Error('Collection name is required');
			const result = await collections.list(selectedCollection, {
				page: pageIndex,
				perPage: pageSize,
				search: search || undefined,
			});
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: !!selectedCollection,
	}));

	const totalPages = $derived(Math.max(1, Math.ceil((docsQuery.data?.total || 0) / pageSize)));

	function handleCollectionSelect(name: string) {
		selectedCollection = name;
		pageIndex = 1;
		search = '';
	}

	function handleRowClick(document: Record<string, any>) {
		editDocument = document;
		drawerOpen = true;
	}

	function handleCreateNew() {
		editDocument = null;
		drawerOpen = true;
	}

	function handleDrawerSuccess() {
		docsQuery.refetch();
	}

	function handleDeleteSuccess() {
		docsQuery.refetch();
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Collections</h1>
			<p class="text-sm text-muted-foreground">Manage your data collections</p>
		</div>
		{#if selectedCollection}
			<Button onclick={handleCreateNew}>
				<PlusIcon class="mr-2 h-4 w-4" />
				Add Record
			</Button>
		{/if}
	</div>

	<div>
		{#if schemaQuery.isPending}
			<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
				{#each Array(4) as _}
					<Skeleton class="h-20 w-full" />
				{/each}
			</div>
		{:else if schemaQuery.data?.collections}
			<CollectionSelector
				collections={schemaQuery.data.collections}
				{selectedCollection}
				onSelect={handleCollectionSelect}
			/>
		{:else}
			<Card.Root>
				<Card.Content class="py-10 text-center">
					<DatabaseIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
					<h3 class="mt-3 text-sm font-medium">No collections defined</h3>
					<p class="mt-1 text-sm text-muted-foreground">
						Add collections to your schema to get started
					</p>
				</Card.Content>
			</Card.Root>
		{/if}
	</div>

	{#if !selectedCollection}
		<Card.Root>
			<Card.Content class="py-10 text-center">
				<DatabaseIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
				<h3 class="mt-3 text-sm font-medium">Select a collection</h3>
				<p class="mt-1 text-sm text-muted-foreground">
					Choose a collection above to view and manage records
				</p>
			</Card.Content>
		</Card.Root>
	{:else if selectedCollectionSchema}
		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<h2 class="text-xl font-semibold tracking-tight">{selectedCollection}</h2>
					{#if collectionDocsUrl}
						<Tooltip.Root>
							<Tooltip.Trigger>
								<a
									href={collectionDocsUrl}
									target="_blank"
									rel="noopener noreferrer"
									class="inline-flex items-center justify-center rounded-md p-1.5 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
								>
									<BookOpenIcon class="h-4 w-4" />
								</a>
							</Tooltip.Trigger>
							<Tooltip.Content>
								<p>View API docs</p>
							</Tooltip.Content>
						</Tooltip.Root>
					{/if}
				</div>
				<div class="flex items-center gap-2">
					<Button variant="outline" size="sm" onclick={() => docsQuery.refetch()}>
						<RefreshCwIcon class="h-3.5 w-3.5 mr-1.5" />
						Refresh
					</Button>
				</div>
			</div>

			<div class="flex items-center gap-4">
				<div class="relative flex-1 max-w-sm">
					<SearchIcon class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
					<Input
						type="search"
						placeholder="Search documents..."
						class="pl-9"
						value={search}
						oninput={(e) => {
							search = e.currentTarget.value;
							pageIndex = 1;
						}}
					/>
				</div>
				{#if docsQuery.data}
					<p class="text-sm text-muted-foreground">
						{docsQuery.data.total} document{docsQuery.data.total === 1 ? '' : 's'}
					</p>
				{/if}
			</div>

			{#if docsQuery.isPending}
				<Skeleton class="h-64 w-full" />
			{:else if docsQuery.isError}
				<Card.Root class="border-destructive/50">
					<Card.Content class="py-4">
						<p class="text-sm text-destructive">
							Failed to load: {docsQuery.error?.message}
						</p>
					</Card.Content>
				</Card.Root>
			{:else if docsQuery.data}
				<div class="space-y-4">
					<CollectionTable
						collection={selectedCollectionSchema}
						documents={docsQuery.data?.docs || []}
						isLoading={docsQuery.isFetching}
						onRowClick={handleRowClick}
						onDeleteSuccess={handleDeleteSuccess}
					/>

					{#if docsQuery.data?.docs && docsQuery.data.docs.length > 0}
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-2 text-sm text-muted-foreground">
								<span>Rows per page</span>
								<select
									class="h-8 w-[70px] rounded-md border border-input bg-background px-2 py-1 text-sm"
									value={String(pageSize)}
									onchange={(e) => {
										pageSize = Number(e.currentTarget.value);
										pageIndex = 1;
									}}
								>
									<option value="10">10</option>
									<option value="25">25</option>
									<option value="50">50</option>
									<option value="100">100</option>
								</select>
							</div>

							<div class="flex items-center gap-4">
								<span class="text-sm text-muted-foreground">
									Page {pageIndex} of {totalPages}
								</span>
								<div class="flex items-center gap-1">
									<Button
										variant="outline"
										size="icon"
										class="h-8 w-8"
										onclick={() => (pageIndex = 1)}
										disabled={pageIndex === 1}
									>
										<ChevronsLeftIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="outline"
										size="icon"
										class="h-8 w-8"
										onclick={() => pageIndex--}
										disabled={pageIndex === 1}
									>
										<ChevronLeftIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="outline"
										size="icon"
										class="h-8 w-8"
										onclick={() => pageIndex++}
										disabled={pageIndex >= totalPages}
									>
										<ChevronRightIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="outline"
										size="icon"
										class="h-8 w-8"
										onclick={() => (pageIndex = totalPages)}
										disabled={pageIndex >= totalPages}
									>
										<ChevronsRightIcon class="h-4 w-4" />
									</Button>
								</div>
							</div>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>

{#if selectedCollectionSchema}
	<CollectionDrawer
		collection={selectedCollectionSchema}
		document={editDocument}
		bind:open={drawerOpen}
		onSuccess={handleDrawerSuccess}
	/>
{/if}

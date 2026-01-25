<script lang="ts">
  import { page } from '$app/stores';
  import { createQuery } from '@tanstack/svelte-query';
  import { collections, admin, type Collection } from '$lib/api/client';
  import { configStore } from '$lib/stores/config.svelte';
  import { CollectionDrawer, CollectionTable } from '$lib/components/collections';
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

  const collectionName = $derived($page.params.collection);
  const collectionDocsUrl = $derived(collectionName ? configStore.getCollectionDocsUrl(collectionName) : null);

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

  const collectionSchema = $derived.by(() => {
    if (!schemaQuery.data || !collectionName) return null;
    return schemaQuery.data.collections?.find((c) => c.name === collectionName) || null;
  });

  const docsQuery = createQuery(() => ({
    queryKey: ['collections', collectionName, 'documents', pageIndex, pageSize, search],
    queryFn: async () => {
      if (!collectionName) throw new Error('Collection name is required');
      const result = await collections.list(collectionName, {
        page: pageIndex,
        perPage: pageSize,
        search: search || undefined,
      });
      if (result.error) throw new Error(result.error.message);
      return result.data!;
    },
    enabled: !!collectionName,
  }));

  const totalPages = $derived(Math.max(1, Math.ceil((docsQuery.data?.total || 0) / pageSize)));

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

<div class="space-y-6">
  <div class="flex items-center justify-between mb-6">
    <div>
      <div class="flex items-center gap-2">
        <h1 class="text-2xl font-semibold tracking-tight">{collectionName}</h1>
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
      <p class="text-sm text-muted-foreground">
        {#if docsQuery.data}
          {docsQuery.data.total} document{docsQuery.data.total === 1 ? '' : 's'}
        {:else}
          Loading...
        {/if}
      </p>
    </div>
    <div class="flex items-center gap-2">
      <Button size="sm" onclick={handleCreateNew}>
        <PlusIcon class="h-3.5 w-3.5 mr-1.5" />
        New Document
      </Button>
      <Button variant="outline" size="sm" onclick={() => docsQuery.refetch()}>
        <RefreshCwIcon class="h-3.5 w-3.5 mr-1.5" />
        Refresh
      </Button>
    </div>
  </div>

  <div class="flex items-center gap-4 mb-4">
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
  </div>

  {#if docsQuery.isPending || schemaQuery.isPending}
    <Card.Root class="flex items-center justify-center py-16">
      <Skeleton class="h-32 w-32" />
    </Card.Root>
  {:else if docsQuery.isError || schemaQuery.isError || !collectionSchema}
    <Card.Root class="border-destructive/50 flex items-center justify-center py-16">
      <p class="text-sm text-destructive">
        Failed to load: {docsQuery.error?.message || schemaQuery.error?.message || 'Collection not found'}
      </p>
    </Card.Root>
  {:else}
    <Card.Root class="overflow-hidden">
      <CollectionTable
        collection={collectionSchema}
        documents={docsQuery.data?.docs || []}
        isLoading={docsQuery.isFetching}
        onRowClick={handleRowClick}
        onDeleteSuccess={handleDeleteSuccess}
      />

      {#if docsQuery.data?.docs && docsQuery.data.docs.length > 0}
        <div class="flex items-center justify-between px-4 py-3 border-t bg-card">
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
    </Card.Root>
  {/if}

  {#if collectionSchema}
    <CollectionDrawer
      collection={collectionSchema}
      document={editDocument}
      bind:open={drawerOpen}
      onSuccess={handleDrawerSuccess}
    />
  {/if}
</div>

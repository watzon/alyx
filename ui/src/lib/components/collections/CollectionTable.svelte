<script lang="ts">
  import * as Table from '$ui/table';
  import { Skeleton } from '$ui/skeleton';
  import * as Tooltip from '$ui/tooltip';
  import type { Collection } from '$lib/api/client';
  import DeleteButton from './DeleteButton.svelte';

  interface Props {
    collection: Collection;
    documents: Record<string, any>[];
    isLoading?: boolean;
    onRowClick: (document: Record<string, any>) => void;
    onDeleteSuccess?: () => void;
  }

  let { collection, documents, isLoading = false, onRowClick, onDeleteSuccess }: Props = $props();

  function formatDate(dateString: string | null | undefined): string {
    if (!dateString) return '-';
    try {
      return new Date(dateString).toLocaleDateString();
    } catch {
      return '-';
    }
  }

  function formatValue(value: unknown, type: string): string {
    if (value === null || value === undefined) return '-';
    if (type === 'bool') return value ? 'Yes' : 'No';
    if (type === 'timestamp' && typeof value === 'string') {
      return formatDate(value);
    }
    if (typeof value === 'object') {
      return JSON.stringify(value);
    }
    return String(value);
  }

  function truncate(str: string, maxLength: number = 50): string {
    if (str.length <= maxLength) return str;
    return str.slice(0, maxLength) + '...';
  }
</script>

{#if isLoading}
  <div class="space-y-2">
    {#each Array(5) as _, i}
      <Skeleton class="h-12 w-full" />
    {/each}
  </div>
{:else if documents.length === 0}
  <div class="flex flex-col items-center justify-center py-16 text-center">
    <div class="rounded-full bg-muted p-4 mb-4">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-8 w-8 text-muted-foreground"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
        />
      </svg>
    </div>
    <h3 class="text-sm font-medium mb-1">No documents yet</h3>
    <p class="text-sm text-muted-foreground">
      Get started by creating your first document
    </p>
  </div>
{:else}
  <div class="rounded-md border overflow-hidden">
    <Table.Root>
      <Table.Header class="sticky top-0 z-20 bg-card">
        <Table.Row>
          {#each collection.fields || [] as field}
            <Table.Head class="whitespace-nowrap">
              {field.name}
            </Table.Head>
          {/each}
          <Table.Head class="w-[60px]">
            <span class="sr-only">Actions</span>
          </Table.Head>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {#each documents as doc (doc.id)}
          <Table.Row
            class="cursor-pointer hover:bg-muted/30 transition-colors"
            onclick={() => onRowClick(doc)}
          >
            {#each collection.fields || [] as field}
              {@const value = doc[field.name]}
              {@const formattedValue = formatValue(value, field.type)}
              <Table.Cell class="font-mono text-sm">
                {#if formattedValue.length > 50}
                  <Tooltip.Provider>
                    <Tooltip.Root>
                      <Tooltip.Trigger class="text-left">
                        {truncate(formattedValue)}
                      </Tooltip.Trigger>
                      <Tooltip.Content>
                        <p class="max-w-xs">{formattedValue}</p>
                      </Tooltip.Content>
                    </Tooltip.Root>
                  </Tooltip.Provider>
                {:else}
                  {formattedValue}
                {/if}
              </Table.Cell>
            {/each}
            <Table.Cell onclick={(e) => e.stopPropagation()}>
              <DeleteButton
                collectionName={collection.name}
                documentId={doc.id}
                onSuccess={onDeleteSuccess}
              />
            </Table.Cell>
          </Table.Row>
        {/each}
      </Table.Body>
    </Table.Root>
  </div>
{/if}

<script lang="ts">
  import * as Table from '$ui/table';
  import { Badge } from '$ui/badge';
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

  function formatValue(value: unknown, type: string): string {
    if (value === null || value === undefined) return '-';
    if (type === 'bool') return value ? 'Yes' : 'No';
    if (type === 'timestamp' && typeof value === 'string') {
      try {
        return new Date(value).toLocaleString();
      } catch {
        return String(value);
      }
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

  function getTypeColor(type: string): string {
    const colors: Record<string, string> = {
      uuid: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300',
      string: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
      text: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
      int: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
      float: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
      bool: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-300',
      timestamp: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-300',
      json: 'bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-300',
      blob: 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300',
    };
    return colors[type] || 'bg-gray-100 text-gray-800';
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
        <Table.Row class="border-b-0">
          {#each collection.fields || [] as field}
            <Table.Head class="whitespace-nowrap">
              <div class="flex items-center gap-2">
                <span class="text-xs font-medium">{field.name}</span>
                <Badge variant="secondary" class="text-[10px] font-normal {getTypeColor(field.type)}">
                  {field.type}
                </Badge>
              </div>
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

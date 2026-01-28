<script lang="ts">
  import * as Table from '$ui/table';
  import { Skeleton } from '$ui/skeleton';
  import * as Tooltip from '$ui/tooltip';
  import type { Collection, Field } from '$lib/api/client';
  import { files } from '$lib/api/client';
  import DeleteButton from './DeleteButton.svelte';
  import FileIcon from '@lucide/svelte/icons/file';

  interface Props {
    collection: Collection;
    documents: Record<string, any>[];
    isLoading?: boolean;
    onRowClick: (document: Record<string, any>) => void;
    onDeleteSuccess?: () => void;
  }

  let { collection, documents, isLoading = false, onRowClick, onDeleteSuccess }: Props = $props();

  function getFileViewUrl(field: Field, fileId: string): string | null {
    if (!fileId || !field.file?.bucket) return null;
    return files.getViewUrl(field.file.bucket, fileId);
  }

  function isImageFile(field: Field): boolean {
    // Check if the bucket is likely to contain images based on allowed types
    const allowedTypes = field.file?.allowedTypes || [];
    return allowedTypes.some(t => t.startsWith('image/')) || allowedTypes.includes('image/*');
  }

  function formatDate(value: unknown): string {
    if (value === null || value === undefined) return '-';
    try {
      // Handle various date formats
      let date: Date;
      if (typeof value === 'number') {
        // Unix timestamp (seconds or milliseconds)
        date = new Date(value > 1e12 ? value : value * 1000);
      } else if (typeof value === 'string') {
        date = new Date(value);
      } else {
        return '-';
      }
      // Check if date is valid
      if (isNaN(date.getTime())) return '-';
      return date.toLocaleDateString();
    } catch {
      return '-';
    }
  }

  function formatValue(value: unknown, type: string): string {
    if (value === null || value === undefined) return '-';
    if (type === 'bool') return value ? 'Yes' : 'No';
    if (type === 'timestamp' || type === 'datetime') {
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
              <Table.Cell class="font-mono text-sm">
                {#if field.type === 'file' && value}
                  {@const viewUrl = getFileViewUrl(field, value)}
                  {#if viewUrl}
                    <img 
                      src={viewUrl} 
                      alt="" 
                      class="w-8 h-8 object-cover rounded"
                      onerror={(e) => (e.currentTarget as HTMLImageElement).style.display = 'none'}
                    />
                  {:else}
                    <div class="w-8 h-8 rounded bg-muted flex items-center justify-center">
                      <FileIcon class="w-4 h-4 text-muted-foreground" />
                    </div>
                  {/if}
                {:else}
                  {@const formattedValue = formatValue(value, field.type)}
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

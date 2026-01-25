<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { tick } from 'svelte';
  import { collections, type Field } from '$lib/api/client';
  import { Button } from '$lib/components/ui/button';
  import * as Popover from '$lib/components/ui/popover';
  import * as Command from '$lib/components/ui/command';
  import CheckIcon from 'lucide-svelte/icons/check';
  import ChevronsUpDownIcon from 'lucide-svelte/icons/chevrons-up-down';
  import XIcon from 'lucide-svelte/icons/x';
  import LoaderIcon from 'lucide-svelte/icons/loader';
  import { cn } from '$lib/utils';

  interface Props {
    field: Field;
    value: string | null;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  let open = $state(false);
  let searchQuery = $state('');
  let triggerRef = $state<HTMLButtonElement>(null!);

  const relationConfig = $derived(field.relation);
  const referencedCollection = $derived(relationConfig?.collection || '');
  const referencedField = $derived(relationConfig?.field || 'id');
  const displayField = $derived(relationConfig?.displayName || '');

  const recordsQuery = createQuery(() => ({
    queryKey: ['collections', referencedCollection, 'records', searchQuery],
    queryFn: async () => {
      if (!referencedCollection) return { docs: [], total: 0 };
      const result = await collections.list(referencedCollection, {
        perPage: 50,
        search: searchQuery || undefined,
      });
      if (result.error) throw new Error(result.error.message);
      return result.data!;
    },
    enabled: !!referencedCollection,
  }));

  function getDisplayValue(record: Record<string, unknown>): string {
    if (displayField && record[displayField]) {
      return String(record[displayField]);
    }
    const displayFields = ['name', 'title', 'label', 'email', 'username', referencedField, 'id'];
    for (const fieldName of displayFields) {
      if (record[fieldName] && typeof record[fieldName] === 'string') {
        return record[fieldName] as string;
      }
    }
    return String(record.id || 'Unknown');
  }

  const selectedRecord = $derived.by(() => {
    if (!value) return null;
    return recordsQuery.data?.docs?.find((r) => r[referencedField] === value) || null;
  });

  const displayValue = $derived(selectedRecord ? getDisplayValue(selectedRecord) : value);

  function selectRecord(recordId: string) {
    value = recordId;
    closeAndFocusTrigger();
  }

  function clearSelection(e: MouseEvent) {
    e.stopPropagation();
    value = null;
    searchQuery = '';
  }

  function closeAndFocusTrigger() {
    open = false;
    searchQuery = '';
    tick().then(() => {
      triggerRef?.focus();
    });
  }
</script>

<div>
  <Popover.Root bind:open>
    <Popover.Trigger bind:ref={triggerRef}>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="outline"
          class={cn(
            "w-full justify-between font-mono text-sm",
            !displayValue && "text-muted-foreground",
            errors?.length && "border-destructive"
          )}
          role="combobox"
          aria-expanded={open}
          {disabled}
        >
          <span class="truncate">
            {displayValue || `Select ${referencedCollection}...`}
          </span>
          <div class="flex items-center gap-1 shrink-0">
            {#if value && !disabled}
              <button
                type="button"
                class="p-0.5 rounded hover:bg-muted"
                onclick={clearSelection}
              >
                <XIcon class="h-3.5 w-3.5" />
              </button>
            {/if}
            <ChevronsUpDownIcon class="h-4 w-4 opacity-50" />
          </div>
        </Button>
      {/snippet}
    </Popover.Trigger>

    <Popover.Content class="w-(--bits-popover-anchor-width) p-0" align="start">
      <Command.Root shouldFilter={false}>
        <Command.Input 
          placeholder={`Search ${referencedCollection}...`}
          bind:value={searchQuery}
        />
        <Command.List>
          {#if recordsQuery.isPending}
            <Command.Loading>
              <div class="flex items-center justify-center py-6 text-sm text-muted-foreground">
                <LoaderIcon class="h-4 w-4 animate-spin mr-2" />
                Loading...
              </div>
            </Command.Loading>
          {:else if recordsQuery.isError}
            <Command.Empty>
              <span class="text-destructive">Failed to load records</span>
            </Command.Empty>
          {:else if !recordsQuery.data?.docs?.length}
            <Command.Empty>No records found</Command.Empty>
          {:else}
            <Command.Group>
              {#each recordsQuery.data.docs as record (record.id)}
                {@const recordId = String(record[referencedField])}
                <Command.Item
                  value={recordId}
                  onSelect={() => selectRecord(recordId)}
                >
                  <CheckIcon
                    class={cn(
                      "mr-2 h-4 w-4",
                      value === recordId ? "opacity-100" : "opacity-0"
                    )}
                  />
                  <span class="truncate flex-1">{getDisplayValue(record)}</span>
                  <span class="text-xs text-muted-foreground font-mono shrink-0 ml-2">
                    {recordId.length > 8 ? `${recordId.slice(0, 8)}...` : recordId}
                  </span>
                </Command.Item>
              {/each}
            </Command.Group>
          {/if}
        </Command.List>
      </Command.Root>
    </Popover.Content>
  </Popover.Root>

  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

<script lang="ts">
  import * as Select from '$lib/components/ui/select';
  import { Badge } from '$lib/components/ui/badge';
  import type { Field } from '$lib/api/client';
  import XIcon from 'lucide-svelte/icons/x';

  interface Props {
    field: Field;
    value: string | string[];
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  const selectConfig = $derived(field.select as { values: string[]; maxSelect?: number } | undefined);
  const options = $derived(selectConfig?.values ?? []);
  const maxSelect = $derived(selectConfig?.maxSelect ?? 1);
  const isMultiple = $derived(maxSelect !== 1);

  const selectedValues = $derived.by(() => {
    if (!value) return [];
    if (Array.isArray(value)) return value;
    return [value];
  });

  function handleSingleChange(val: string | undefined) {
    if (val) {
      value = val;
    }
  }

  function handleMultiChange(vals: string[]) {
    value = vals;
  }

  function removeValue(val: string) {
    if (Array.isArray(value)) {
      value = value.filter(v => v !== val);
    }
  }
</script>

{#if isMultiple}
  <div class="space-y-2">
    <Select.Root 
      type="multiple" 
      value={selectedValues} 
      onValueChange={handleMultiChange}
      {disabled}
    >
      <Select.Trigger id={field.name} class="w-full {errors?.length ? 'border-destructive' : ''}">
        {#if selectedValues.length === 0}
          Select options...
        {:else}
          {selectedValues.length} selected
        {/if}
      </Select.Trigger>
      <Select.Content>
        {#each options as option}
          <Select.Item value={option}>{option}</Select.Item>
        {/each}
      </Select.Content>
    </Select.Root>
    
    {#if selectedValues.length > 0}
      <div class="flex flex-wrap gap-1">
        {#each selectedValues as val}
          <Badge variant="secondary" class="gap-1 pr-1">
            {val}
            {#if !disabled}
              <button
                type="button"
                onclick={() => removeValue(val)}
                class="hover:bg-muted rounded-sm"
              >
                <XIcon class="h-3 w-3" />
              </button>
            {/if}
          </Badge>
        {/each}
      </div>
    {/if}
    
    {#if maxSelect > 0}
      <p class="text-xs text-muted-foreground">
        {selectedValues.length}/{maxSelect} selections
      </p>
    {/if}
  </div>
{:else}
  <Select.Root 
    type="single" 
    value={Array.isArray(value) ? value[0] : value} 
    onValueChange={handleSingleChange}
    {disabled}
  >
    <Select.Trigger id={field.name} class="w-full {errors?.length ? 'border-destructive' : ''}">
      {value || `Select ${field.name}`}
    </Select.Trigger>
    <Select.Content>
      {#each options as option}
        <Select.Item value={option}>{option}</Select.Item>
      {/each}
    </Select.Content>
  </Select.Root>
{/if}

{#if errors?.length}
  <p class="text-sm text-destructive mt-1.5">{errors[0]}</p>
{/if}

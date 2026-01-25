<script lang="ts">
  import { Textarea } from '$lib/components/ui/textarea';
  import { Button } from '$lib/components/ui/button';
  import WandIcon from 'lucide-svelte/icons/wand';
  import { cn } from '$lib/utils';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: any;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  let jsonString = $state(value ? JSON.stringify(value, null, 2) : '');
  let parseError = $state<string | null>(null);

  $effect(() => {
    if (!jsonString.trim()) {
      value = null;
      parseError = null;
      return;
    }

    try {
      value = JSON.parse(jsonString);
      parseError = null;
    } catch (err) {
      parseError = err instanceof Error ? err.message : 'Invalid JSON';
    }
  });

  function formatJson() {
    try {
      const parsed = JSON.parse(jsonString);
      jsonString = JSON.stringify(parsed, null, 2);
    } catch {
      // Ignore format if invalid
    }
  }
</script>

<div>
  <div class="flex items-center justify-end mb-2">
    <Button variant="ghost" size="sm" class="h-7 text-xs" onclick={formatJson} {disabled}>
      <WandIcon class="h-3 w-3 mr-1" />
      Format
    </Button>
  </div>

  <Textarea
    id={field.name}
    bind:value={jsonString}
    {disabled}
    placeholder={`{\n  "key": "value"\n}`}
    class={cn(
      "font-mono text-sm min-h-[150px]",
      (errors?.length || parseError) && "border-destructive"
    )}
  />

  {#if parseError}
    <p class="text-sm text-destructive mt-2">{parseError}</p>
  {:else if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

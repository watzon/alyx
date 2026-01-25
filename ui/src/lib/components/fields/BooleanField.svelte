<script lang="ts">
  import { Switch } from '$lib/components/ui/switch';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: boolean;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  // Ensure value is always a boolean (handle undefined from form initialization race)
  const checked = $derived(value ?? false);
</script>

<div>
  <div class="flex items-center gap-3">
    <Switch
      id={field.name}
      checked={checked}
      onCheckedChange={(v) => (value = v)}
      {disabled}
    />
    <label for={field.name} class="text-sm cursor-pointer select-none {checked ? 'text-foreground' : 'text-muted-foreground'}">
      {checked ? 'Enabled' : 'Disabled'}
    </label>
  </div>
  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

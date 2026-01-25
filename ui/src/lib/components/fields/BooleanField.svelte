<script lang="ts">
  import { Switch } from '$lib/components/ui/switch';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: boolean;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(false), errors, disabled = false }: Props = $props();
</script>

<div>
  <div class="flex items-center gap-3">
    <Switch
      id={field.name}
      bind:checked={value}
      {disabled}
    />
    <label for={field.name} class="text-sm cursor-pointer select-none {value ? 'text-foreground' : 'text-muted-foreground'}">
      {value ? 'Enabled' : 'Disabled'}
    </label>
  </div>
  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

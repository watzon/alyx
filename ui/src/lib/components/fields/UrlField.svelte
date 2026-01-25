<script lang="ts">
  import { Input } from '$lib/components/ui/input';
  import type { Field } from '$lib/api/client';
  import LinkIcon from 'lucide-svelte/icons/link';

  interface Props {
    field: Field;
    value: string;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  const maxLength = $derived(field.validate?.maxLength as number | undefined);
</script>

<div class="relative">
  <LinkIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
  <Input
    id={field.name}
    type="url"
    bind:value
    {disabled}
    placeholder={field.nullable ? 'https://example.com' : 'https://example.com (required)'}
    maxlength={maxLength}
    class="pl-10 {errors?.length ? 'border-destructive' : ''}"
  />
</div>
{#if errors?.length}
  <p class="text-sm text-destructive mt-1.5">{errors[0]}</p>
{/if}

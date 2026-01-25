<script lang="ts">
  import { Input } from '$lib/components/ui/input';
  import { Slider } from '$lib/components/ui/slider';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: number | null;
    errors?: string[];
    disabled?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false }: Props = $props();

  const min = $derived(field.validate?.min as number | undefined);
  const max = $derived(field.validate?.max as number | undefined);
  const step = $derived(field.type === 'float' ? 0.01 : 1);
  const useSlider = $derived(field.validate?.slider === true && min !== undefined && max !== undefined);
  
  let sliderValue = $state<number[]>(value !== null && value !== undefined ? [value] : [min ?? 0]);
  
  $effect(() => {
    if (useSlider && sliderValue[0] !== value) {
      value = sliderValue[0];
    }
  });

  $effect(() => {
    if (useSlider && value !== null && value !== undefined && sliderValue[0] !== value) {
      sliderValue = [value];
    }
  });
</script>

<div>
  {#if useSlider}
    <div class="space-y-3">
      <div class="flex items-center gap-4">
        <Slider
          type="single"
          bind:value={sliderValue}
          {min}
          {max}
          {step}
          {disabled}
          class="flex-1"
        />
        <span class="text-sm font-medium tabular-nums min-w-[3rem] text-right">
          {sliderValue[0]}
        </span>
      </div>
      <div class="flex justify-between text-xs text-muted-foreground">
        <span>{min}</span>
        <span>{max}</span>
      </div>
    </div>
  {:else}
    <Input
      id={field.name}
      type="number"
      bind:value
      {disabled}
      {min}
      {max}
      {step}
      placeholder={field.nullable ? 'Optional' : 'Required'}
      class={errors?.length ? 'border-destructive' : ''}
    />
    {#if min !== undefined || max !== undefined}
      <p class="text-xs text-muted-foreground mt-1.5">
        {#if min !== undefined && max !== undefined}
          Range: {min} - {max}
        {:else if min !== undefined}
          Minimum: {min}
        {:else if max !== undefined}
          Maximum: {max}
        {/if}
      </p>
    {/if}
  {/if}
  {#if errors?.length}
    <p class="text-sm text-destructive mt-1.5">{errors[0]}</p>
  {/if}
</div>

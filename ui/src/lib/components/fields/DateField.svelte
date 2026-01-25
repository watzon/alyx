<script lang="ts">
  import { Calendar } from '$lib/components/ui/calendar';
  import * as Popover from '$lib/components/ui/popover';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import CalendarIcon from 'lucide-svelte/icons/calendar';
  import XIcon from 'lucide-svelte/icons/x';
  import { CalendarDate, parseDate, getLocalTimeZone } from '@internationalized/date';
  import type { Field } from '$lib/api/client';

  interface Props {
    field: Field;
    value: string | null;
    errors?: string[];
    disabled?: boolean;
    dateOnly?: boolean;
  }

  let { field, value = $bindable(), errors, disabled = false, dateOnly = false }: Props = $props();

  const hasAutoDefault = $derived(field.default === 'now' || field.default === 'CURRENT_TIMESTAMP');
  const isOptional = $derived(field.nullable || hasAutoDefault);

  function getPlaceholder(): string {
    if (hasAutoDefault) return 'Auto (current time)';
    if (field.nullable) return 'Optional';
    return 'YYYY-MM-DD';
  }

  function parseInitialValue(val: string | null): { input: string; calendar: CalendarDate | undefined } {
    if (!val || val === 'now' || val === 'CURRENT_TIMESTAMP') {
      return { input: '', calendar: undefined };
    }
    
    try {
      const dateStr = val.split('T')[0];
      return { 
        input: dateStr, 
        calendar: parseDate(dateStr) 
      };
    } catch {
      return { input: '', calendar: undefined };
    }
  }

  const initialParsed = parseInitialValue(value);
  let open = $state(false);
  let inputValue = $state(initialParsed.input);
  let calendarValue = $state<CalendarDate | undefined>(initialParsed.calendar);

  $effect(() => {
    if (calendarValue) {
      const date = calendarValue.toDate(getLocalTimeZone());
      inputValue = date.toISOString().split('T')[0];
      value = dateOnly ? inputValue : date.toISOString();
    }
  });

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    inputValue = target.value;
    
    try {
      if (inputValue && inputValue.match(/^\d{4}-\d{2}-\d{2}$/)) {
        calendarValue = parseDate(inputValue);
        const date = calendarValue.toDate(getLocalTimeZone());
        value = dateOnly ? inputValue : date.toISOString();
      } else if (!inputValue) {
        calendarValue = undefined;
        value = null;
      }
    } catch {
      // Keep inputValue on invalid date format
    }
  }

  function clearDate() {
    calendarValue = undefined;
    value = null;
    inputValue = '';
  }
</script>

<div>
  <div class="flex gap-2">
    <Popover.Root bind:open>
      <div class="relative flex-1">
        <Input
          id={field.name}
          type="text"
          value={inputValue}
          oninput={handleInput}
          {disabled}
          placeholder={getPlaceholder()}
          class={errors?.length ? 'border-destructive pe-10' : 'pe-10'}
        />
        <div class="absolute right-1 top-1/2 -translate-y-1/2 flex items-center">
          <Popover.Trigger>
            {#snippet child({ props })}
              <button
                type="button"
                class="size-7 rounded hover:bg-muted inline-flex items-center justify-center disabled:opacity-50 disabled:pointer-events-none"
                {disabled}
                {...props}
              >
                <CalendarIcon class="h-4 w-4 text-muted-foreground" />
              </button>
            {/snippet}
          </Popover.Trigger>
        </div>
      </div>

      <Popover.Content class="w-auto p-0" align="start">
        <Calendar 
          bind:value={calendarValue} 
          type="single" 
          onValueChange={() => {
            open = false;
          }}
        />
      </Popover.Content>
    </Popover.Root>

    {#if isOptional && (calendarValue || inputValue)}
      <Button variant="ghost" size="icon" onclick={clearDate} {disabled}>
        <XIcon class="h-4 w-4" />
      </Button>
    {/if}
  </div>

  {#if hasAutoDefault && !inputValue}
    <p class="text-xs text-muted-foreground mt-1.5">Will be set automatically if left empty</p>
  {/if}
  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

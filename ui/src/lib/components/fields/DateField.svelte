<script lang="ts">
  import { Calendar } from '$lib/components/ui/calendar';
  import * as Popover from '$lib/components/ui/popover';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import CalendarIcon from 'lucide-svelte/icons/calendar';
  import XIcon from 'lucide-svelte/icons/x';
  import { CalendarDate, parseDate, getLocalTimeZone } from '@internationalized/date';
  import type { Field } from '$lib/api/client';
  import { formatDateTime } from '$lib/utils/datetime';

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
    return dateOnly ? 'YYYY-MM-DD' : 'YYYY-MM-DD HH:MM';
  }

  function parseValue(val: string | null): { dateStr: string; timeStr: string; calendar: CalendarDate | undefined } {
    if (!val || val === 'now' || val === 'CURRENT_TIMESTAMP') {
      return { dateStr: '', timeStr: '', calendar: undefined };
    }
    
    try {
      const d = new Date(val);
      if (isNaN(d.getTime())) {
        return { dateStr: '', timeStr: '', calendar: undefined };
      }
      
      const year = d.getFullYear();
      const month = String(d.getMonth() + 1).padStart(2, '0');
      const day = String(d.getDate()).padStart(2, '0');
      const hours = String(d.getHours()).padStart(2, '0');
      const minutes = String(d.getMinutes()).padStart(2, '0');
      
      const dateStr = `${year}-${month}-${day}`;
      const timeStr = `${hours}:${minutes}`;
      
      return { 
        dateStr, 
        timeStr,
        calendar: parseDate(dateStr) 
      };
    } catch {
      return { dateStr: '', timeStr: '', calendar: undefined };
    }
  }

  let open = $state(false);
  let dateInput = $state('');
  let timeInput = $state('');
  let calendarValue = $state<CalendarDate | undefined>(undefined);
  let lastPropValue = $state<string | null>(null);

  $effect(() => {
    if (value !== lastPropValue) {
      lastPropValue = value;
      const parsed = parseValue(value);
      dateInput = parsed.dateStr;
      timeInput = parsed.timeStr;
      calendarValue = parsed.calendar;
    }
  });

  function updateValue() {
    if (!dateInput) {
      value = null;
      return;
    }
    
    try {
      if (dateOnly) {
        value = dateInput;
      } else {
        const time = timeInput || '00:00';
        const [hours, minutes] = time.split(':').map(Number);
        const d = new Date(dateInput);
        d.setHours(hours || 0, minutes || 0, 0, 0);
        const newValue = d.toISOString();
        if (value !== newValue) {
          value = newValue;
          lastPropValue = newValue;
        }
      }
    } catch {
      // Invalid date
    }
  }

  $effect(() => {
    if (calendarValue) {
      const date = calendarValue.toDate(getLocalTimeZone());
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const newDateInput = `${year}-${month}-${day}`;
      
      if (dateInput !== newDateInput) {
        dateInput = newDateInput;
        updateValue();
      }
    }
  });

  function handleDateInput(e: Event) {
    const target = e.target as HTMLInputElement;
    dateInput = target.value;
    
    try {
      if (dateInput && dateInput.match(/^\d{4}-\d{2}-\d{2}$/)) {
        calendarValue = parseDate(dateInput);
      } else if (!dateInput) {
        calendarValue = undefined;
      }
    } catch {
      // Keep dateInput on invalid date format
    }
    
    updateValue();
  }

  function handleTimeInput(e: Event) {
    const target = e.target as HTMLInputElement;
    timeInput = target.value;
    updateValue();
  }

  function clearDate() {
    calendarValue = undefined;
    value = null;
    dateInput = '';
    timeInput = '';
    lastPropValue = null;
  }

  const displayValue = $derived(
    value && value !== 'now' && value !== 'CURRENT_TIMESTAMP' 
      ? formatDateTime(value) 
      : ''
  );
</script>

<div>
  <div class="flex gap-2">
    <Popover.Root bind:open>
      <div class="relative flex-1">
        <Input
          id={field.name}
          type="text"
          value={dateInput}
          oninput={handleDateInput}
          {disabled}
          placeholder={dateOnly ? getPlaceholder() : 'YYYY-MM-DD'}
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

    {#if !dateOnly}
      <Input
        type="time"
        value={timeInput}
        oninput={handleTimeInput}
        {disabled}
        placeholder="HH:MM"
        class="w-28 {errors?.length ? 'border-destructive' : ''}"
      />
    {/if}

    {#if isOptional && (dateInput || timeInput)}
      <Button variant="ghost" size="icon" onclick={clearDate} {disabled}>
        <XIcon class="h-4 w-4" />
      </Button>
    {/if}
  </div>

  {#if displayValue && !dateOnly}
    <p class="text-xs text-muted-foreground mt-1.5">{displayValue}</p>
  {/if}
  {#if hasAutoDefault && !dateInput}
    <p class="text-xs text-muted-foreground mt-1.5">Will be set automatically if left empty</p>
  {/if}
  {#if errors?.length}
    <p class="text-sm text-destructive mt-2">{errors[0]}</p>
  {/if}
</div>

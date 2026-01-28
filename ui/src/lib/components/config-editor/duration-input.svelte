<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import * as Select from '$ui/select';
	import { cn } from '$lib/utils.js';

	interface Props {
		value: string;
		onchange: (value: string) => void;
		disabled?: boolean;
		placeholder?: string;
		class?: string;
	}

	let {
		value = $bindable(),
		onchange,
		disabled = false,
		placeholder = '0',
		class: className
	}: Props = $props();

	type Unit = 's' | 'm' | 'h' | 'd';

	interface ParsedDuration {
		hours: number;
		minutes: number;
		seconds: number;
		days: number;
	}

	const units: { value: Unit; label: string }[] = [
		{ value: 's', label: 'Seconds' },
		{ value: 'm', label: 'Minutes' },
		{ value: 'h', label: 'Hours' },
		{ value: 'd', label: 'Days' }
	];

	function parseDuration(duration: string): ParsedDuration {
		const result: ParsedDuration = { hours: 0, minutes: 0, seconds: 0, days: 0 };
		if (!duration) return result;

		// Match patterns like "1h30m", "5s", "2d", etc.
		const regex = /(\d+)([hmsd])/g;
		let match;

		while ((match = regex.exec(duration)) !== null) {
			const num = parseInt(match[1], 10);
			const unit = match[2] as Unit;

			switch (unit) {
				case 'd':
					result.days = num;
					break;
				case 'h':
					result.hours = num;
					break;
				case 'm':
					result.minutes = num;
					break;
				case 's':
					result.seconds = num;
					break;
			}
		}

		return result;
	}

	function formatDuration(parsed: ParsedDuration): string {
		const parts: string[] = [];
		if (parsed.days > 0) parts.push(`${parsed.days}d`);
		if (parsed.hours > 0) parts.push(`${parsed.hours}h`);
		if (parsed.minutes > 0) parts.push(`${parsed.minutes}m`);
		if (parsed.seconds > 0) parts.push(`${parsed.seconds}s`);
		return parts.join('') || '0s';
	}

	function getTotalSeconds(parsed: ParsedDuration): number {
		return (
			parsed.days * 24 * 60 * 60 +
			parsed.hours * 60 * 60 +
			parsed.minutes * 60 +
			parsed.seconds
		);
	}

	function getDisplayValue(): { amount: string; unit: Unit } {
		const parsed = parseDuration(value);
		const totalSeconds = getTotalSeconds(parsed);

		if (totalSeconds === 0) {
			return { amount: '', unit: 's' };
		}

		// Determine the most appropriate unit
		if (parsed.days > 0 && parsed.hours === 0 && parsed.minutes === 0 && parsed.seconds === 0) {
			return { amount: String(parsed.days), unit: 'd' };
		}
		if (parsed.hours > 0 && parsed.days === 0 && parsed.minutes === 0 && parsed.seconds === 0) {
			return { amount: String(parsed.hours), unit: 'h' };
		}
		if (parsed.minutes > 0 && parsed.days === 0 && parsed.hours === 0 && parsed.seconds === 0) {
			return { amount: String(parsed.minutes), unit: 'm' };
		}
		if (parsed.seconds > 0 && parsed.days === 0 && parsed.hours === 0 && parsed.minutes === 0) {
			return { amount: String(parsed.seconds), unit: 's' };
		}

		// Mixed duration - convert to seconds for editing
		return { amount: String(totalSeconds), unit: 's' };
	}

	function handleAmountChange(newAmount: string) {
		const num = parseInt(newAmount, 10);
		if (isNaN(num) || num < 0) {
			onchange('');
			return;
		}

		const { unit } = getDisplayValue();
		const newDuration = `${num}${unit}`;
		onchange(newDuration);
	}

	function handleUnitChange(newUnit: Unit) {
		const { amount } = getDisplayValue();
		const num = parseInt(amount, 10);
		if (isNaN(num) || num <= 0) {
			onchange('');
			return;
		}
		onchange(`${num}${newUnit}`);
	}

	const displayValue = $derived(getDisplayValue());
	const selectedUnit = $derived(units.find((u) => u.value === displayValue.unit));
</script>

<div class={cn('flex items-center gap-2', className)}>
	<Input
		type="number"
		min="0"
		{placeholder}
		value={displayValue.amount}
		onchange={(e) => handleAmountChange(e.currentTarget.value)}
		{disabled}
		class="w-24"
	/>
	<Select.Root
		type="single"
		value={displayValue.unit}
		onValueChange={(v) => handleUnitChange(v as Unit)}
		disabled={disabled || !displayValue.amount}
	>
		<Select.Trigger class="w-28">
			{selectedUnit?.label ?? 'Seconds'}
		</Select.Trigger>
		<Select.Content>
			{#each units as unit}
				<Select.Item value={unit.value}>{unit.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
</div>

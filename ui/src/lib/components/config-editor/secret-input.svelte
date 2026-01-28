<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { cn } from '$lib/utils.js';
	import { toast } from 'svelte-sonner';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LockIcon from '@lucide/svelte/icons/lock';

	interface Props {
		value: string;
		onchange: (value: string) => void;
		disabled?: boolean;
		placeholder?: string;
		isSet?: boolean;
		class?: string;
	}

	let {
		value = $bindable(),
		onchange,
		disabled = false,
		placeholder = 'Enter secret...',
		isSet = false,
		class: className
	}: Props = $props();

	let showValue = $state(false);

	function toggleVisibility() {
		showValue = !showValue;
	}

	async function copyToClipboard() {
		try {
			await navigator.clipboard.writeText(value);
			toast.success('Copied to clipboard');
		} catch {
			toast.error('Failed to copy');
		}
	}

	const displayPlaceholder = $derived(isSet && !value ? '••••••••' : placeholder);
	const inputType = $derived(showValue ? 'text' : 'password');
	const isEmpty = $derived(!value && !isSet);
</script>

<div class={cn('relative flex items-center gap-2', className)}>
	<div class="relative flex-1">
		<Input
			type={inputType}
			{placeholder}
			value={isSet && !value ? '' : value}
			onchange={(e) => onchange(e.currentTarget.value)}
			{disabled}
			class="pr-20"
		/>
		{#if isSet && !value}
			<div
				class="absolute inset-y-0 left-3 flex items-center text-muted-foreground pointer-events-none"
			>
				{displayPlaceholder}
			</div>
		{/if}
		<div class="absolute inset-y-0 right-0 flex items-center pr-1">
			{#if isSet && !value}
				<div class="flex items-center gap-1 px-2">
					<LockIcon class="h-3.5 w-3.5 text-muted-foreground" />
					<span class="text-xs text-muted-foreground">Set</span>
				</div>
			{/if}
			<Button
				variant="ghost"
				size="icon"
				class="h-7 w-7"
				onclick={toggleVisibility}
				disabled={disabled || isEmpty}
				type="button"
				aria-label={showValue ? 'Hide value' : 'Show value'}
			>
				{#if showValue}
					<EyeOffIcon class="h-4 w-4" />
				{:else}
					<EyeIcon class="h-4 w-4" />
				{/if}
			</Button>
		</div>
	</div>
	<Button
		variant="outline"
		size="icon"
		class="h-9 w-9 shrink-0"
		onclick={copyToClipboard}
		disabled={disabled || isEmpty}
		type="button"
		aria-label="Copy to clipboard"
	>
		<CopyIcon class="h-4 w-4" />
	</Button>
</div>

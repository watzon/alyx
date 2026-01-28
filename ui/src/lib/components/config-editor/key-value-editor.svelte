<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { Label } from '$ui/label';
	import { cn } from '$lib/utils.js';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TrashIcon from '@lucide/svelte/icons/trash-2';

	interface Props {
		entries: Record<string, string>;
		onchange: (entries: Record<string, string>) => void;
		disabled?: boolean;
		keyPlaceholder?: string;
		valuePlaceholder?: string;
		label?: string;
		class?: string;
	}

	let {
		entries = $bindable({}),
		onchange,
		disabled = false,
		keyPlaceholder = 'Key',
		valuePlaceholder = 'Value',
		label,
		class: className
	}: Props = $props();

	let newKey = $state('');
	let newValue = $state('');
	let duplicateKeyError = $state<string | null>(null);

	const entryList = $derived(Object.entries(entries));
	const isEmpty = $derived(entryList.length === 0);

	function validateKey(key: string): boolean {
		if (!key.trim()) return false;
		if (key in entries) {
			duplicateKeyError = `Key "${key}" already exists`;
			return false;
		}
		duplicateKeyError = null;
		return true;
	}

	function addEntry() {
		const trimmedKey = newKey.trim();
		if (!validateKey(trimmedKey)) return;

		const updated = { ...entries, [trimmedKey]: newValue.trim() };
		onchange(updated);
		newKey = '';
		newValue = '';
		duplicateKeyError = null;
	}

	function removeEntry(key: string) {
		const updated = { ...entries };
		delete updated[key];
		onchange(updated);
	}

	function updateEntry(oldKey: string, newKeyVal: string, newVal: string) {
		const trimmedNewKey = newKeyVal.trim();
		if (!trimmedNewKey) return;

		// If key changed, check for duplicates
		if (trimmedNewKey !== oldKey && trimmedNewKey in entries) {
			duplicateKeyError = `Key "${trimmedNewKey}" already exists`;
			return;
		}

		const updated = { ...entries };
		delete updated[oldKey];
		updated[trimmedNewKey] = newVal.trim();
		onchange(updated);
		duplicateKeyError = null;
	}

	function updateValue(key: string, newVal: string) {
		const updated = { ...entries, [key]: newVal.trim() };
		onchange(updated);
	}

	function handleNewKeyInput() {
		duplicateKeyError = null;
	}
</script>

<div class={cn('space-y-3', className)}>
	{#if label}
		<Label class="text-sm font-medium">{label}</Label>
	{/if}

	<div class="space-y-2">
		{#if isEmpty}
			<p class="text-sm text-muted-foreground italic py-2">No entries added yet</p>
		{/if}

		{#each entryList as [key, value] (key)}
			<div class="flex items-center gap-2 group">
				<Input
					value={key}
					onchange={(e) => updateEntry(key, e.currentTarget.value, value)}
					placeholder={keyPlaceholder}
					{disabled}
					class="flex-1 font-mono text-sm"
				/>
				<Input
					value={value}
					onchange={(e) => updateValue(key, e.currentTarget.value)}
					placeholder={valuePlaceholder}
					{disabled}
					class="flex-1"
				/>
				<Button
					variant="ghost"
					size="icon"
					class="h-9 w-9 text-destructive hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-opacity"
					onclick={() => removeEntry(key)}
					{disabled}
					type="button"
					aria-label="Remove entry"
				>
					<TrashIcon class="h-4 w-4" />
				</Button>
			</div>
		{/each}

		<div class="space-y-2 pt-2">
			<div class="flex items-center gap-2">
				<Input
					bind:value={newKey}
					placeholder={keyPlaceholder}
					{disabled}
					class="flex-1 font-mono text-sm"
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							addEntry();
						}
					}}
					oninput={handleNewKeyInput}
				/>
				<Input
					bind:value={newValue}
					placeholder={valuePlaceholder}
					{disabled}
					class="flex-1"
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							addEntry();
						}
					}}
				/>
				<Button
					variant="outline"
					size="sm"
					onclick={addEntry}
					disabled={disabled || !newKey.trim()}
					type="button"
				>
					<PlusIcon class="h-4 w-4 mr-1" />
					Add
				</Button>
			</div>
			{#if duplicateKeyError}
				<p class="text-sm text-destructive">{duplicateKeyError}</p>
			{/if}
		</div>
	</div>
</div>

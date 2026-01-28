<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { Label } from '$ui/label';
	import { cn } from '$lib/utils.js';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import GripVerticalIcon from '@lucide/svelte/icons/grip-vertical';

	interface Props {
		values: string[];
		onchange: (values: string[]) => void;
		disabled?: boolean;
		placeholder?: string;
		label?: string;
		class?: string;
	}

	let {
		values = $bindable([]),
		onchange,
		disabled = false,
		placeholder = 'Enter value...',
		label,
		class: className
	}: Props = $props();

	let newValue = $state('');
	let dragIndex = $state<number | null>(null);
	let dragOverIndex = $state<number | null>(null);

	function addItem() {
		if (!newValue.trim()) return;
		const updated = [...values, newValue.trim()];
		onchange(updated);
		newValue = '';
	}

	function removeItem(index: number) {
		const updated = [...values];
		updated.splice(index, 1);
		onchange(updated);
	}

	function updateItem(index: number, newVal: string) {
		const updated = [...values];
		updated[index] = newVal;
		onchange(updated);
	}

	function handleDragStart(e: DragEvent, index: number) {
		dragIndex = index;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', String(index));
		}
	}

	function handleDragOver(e: DragEvent, index: number) {
		e.preventDefault();
		if (dragIndex !== null && dragIndex !== index) {
			dragOverIndex = index;
		}
	}

	function handleDragLeave() {
		dragOverIndex = null;
	}

	function handleDrop(e: DragEvent, dropIndex: number) {
		e.preventDefault();
		if (dragIndex === null || dragIndex === dropIndex) {
			dragIndex = null;
			dragOverIndex = null;
			return;
		}

		const updated = [...values];
		const [moved] = updated.splice(dragIndex, 1);
		updated.splice(dropIndex, 0, moved);
		onchange(updated);

		dragIndex = null;
		dragOverIndex = null;
	}

	function handleDragEnd() {
		dragIndex = null;
		dragOverIndex = null;
	}

	function getItemClass(i: number, isDragOver: boolean, isDragging: boolean): string {
		return cn(
			'flex items-center gap-2 group',
			isDragOver && 'border-primary border-2 rounded-md p-1 bg-primary/5',
			isDragging && 'opacity-50'
		);
	}

	const isEmpty = $derived(values.length === 0);
</script>

<div class={cn('space-y-3', className)}>
	{#if label}
		<Label class="text-sm font-medium">{label}</Label>
	{/if}

	<div class="space-y-2">
		{#if values.length === 0}
			<p class="text-sm text-muted-foreground italic py-2">No items added yet</p>
		{/if}

		{#each values as value, i (i)}
			<div
				class={getItemClass(i, dragOverIndex === i, dragIndex === i)}
				draggable={!disabled}
				ondragstart={(e) => handleDragStart(e, i)}
				ondragover={(e) => handleDragOver(e, i)}
				ondragleave={handleDragLeave}
				ondrop={(e) => handleDrop(e, i)}
				ondragend={handleDragEnd}
				role="listitem"
			>
				<div
					class={cn(
						'cursor-grab text-muted-foreground hover:text-foreground active:cursor-grabbing select-none',
						disabled && 'invisible'
					)}
					role="button"
					tabindex="0"
					aria-label="Drag to reorder"
					onkeydown={(e) => {
						if (e.key === 'ArrowUp' && i > 0) {
							e.preventDefault();
							const updated = [...values];
							[updated[i], updated[i - 1]] = [updated[i - 1], updated[i]];
							onchange(updated);
						} else if (e.key === 'ArrowDown' && i < values.length - 1) {
							e.preventDefault();
							const updated = [...values];
							[updated[i], updated[i + 1]] = [updated[i + 1], updated[i]];
							onchange(updated);
						}
					}}
				>
					<GripVerticalIcon class="h-4 w-4" />
				</div>
				<Input
					value={value}
					onchange={(e) => updateItem(i, e.currentTarget.value)}
					{placeholder}
					{disabled}
					class="flex-1"
				/>
				<Button
					variant="ghost"
					size="icon"
					class="h-9 w-9 text-destructive hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition-opacity"
					onclick={() => removeItem(i)}
					{disabled}
					type="button"
					aria-label="Remove item"
				>
					<TrashIcon class="h-4 w-4" />
				</Button>
			</div>
		{/each}

		<div class="flex items-center gap-2 pt-2">
			<Input
				bind:value={newValue}
				{placeholder}
				{disabled}
				class="flex-1"
				onkeydown={(e) => {
					if (e.key === 'Enter') {
						e.preventDefault();
						addItem();
					}
				}}
			/>
			<Button
				variant="outline"
				size="sm"
				onclick={addItem}
				disabled={disabled || !newValue.trim()}
				type="button"
			>
				<PlusIcon class="h-4 w-4 mr-1" />
				Add
			</Button>
		</div>
	</div>
</div>

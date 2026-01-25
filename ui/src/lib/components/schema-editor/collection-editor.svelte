<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import * as Card from '$ui/card';
	import * as Collapsible from '$ui/collapsible';
	import FieldEditor from './field-editor.svelte';
	import RulesEditor from './rules-editor.svelte';
	import IndexEditor from './index-editor.svelte';
	import {
		type EditableCollection,
		type EditableField,
		type EditableIndex,
		type EditableRules,
		createEmptyField
	} from './types';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import TrashIcon from 'lucide-svelte/icons/trash-2';
	import ChevronDownIcon from 'lucide-svelte/icons/chevron-down';
	import ChevronRightIcon from 'lucide-svelte/icons/chevron-right';
	import ShieldIcon from 'lucide-svelte/icons/shield';
	import DatabaseIcon from 'lucide-svelte/icons/database';

	interface Props {
		collection: EditableCollection;
		allCollections: EditableCollection[];
		onupdate: (collection: EditableCollection) => void;
		ondelete: () => void;
		disabled?: boolean;
	}

	let {
		collection,
		allCollections,
		onupdate,
		ondelete,
		disabled = false
	}: Props = $props();

	let rulesOpen = $state(false);
	let indexesOpen = $state(false);
	let draggingFieldId = $state<string | null>(null);
	let dragOverFieldId = $state<string | null>(null);

	function updateName(name: string) {
		onupdate({ ...collection, name });
	}

	function updateFields(fields: EditableField[]) {
		onupdate({ ...collection, fields });
	}

	function handleDragStart(_e: DragEvent, fieldId: string) {
		draggingFieldId = fieldId;
	}

	function handleDragOver(_e: DragEvent, fieldId: string) {
		if (draggingFieldId && draggingFieldId !== fieldId) {
			dragOverFieldId = fieldId;
		}
	}

	function handleDragEnd() {
		draggingFieldId = null;
		dragOverFieldId = null;
	}

	function handleDrop(_e: DragEvent, targetFieldId: string) {
		if (!draggingFieldId || draggingFieldId === targetFieldId) {
			handleDragEnd();
			return;
		}

		const fields = [...collection.fields];
		const dragIndex = fields.findIndex((f) => f._id === draggingFieldId);
		const dropIndex = fields.findIndex((f) => f._id === targetFieldId);

		if (dragIndex === -1 || dropIndex === -1) {
			handleDragEnd();
			return;
		}

		const [draggedField] = fields.splice(dragIndex, 1);
		fields.splice(dropIndex, 0, draggedField);
		updateFields(fields);
		handleDragEnd();
	}

	function updateField(id: string, field: EditableField) {
		const oldField = collection.fields.find((f) => f._id === id);
		const oldName = oldField?.name;
		const newName = field.name;

		// Update field name in composite indexes if it changed
		let newIndexes = collection.indexes;
		if (oldName && newName && oldName !== newName) {
			newIndexes = collection.indexes.map((idx) => ({
				...idx,
				fields: idx.fields.map((f) => (f === oldName ? newName : f))
			}));
		}

		onupdate({
			...collection,
			fields: collection.fields.map((f) => (f._id === id ? field : f)),
			indexes: newIndexes
		});
	}

	function deleteField(id: string) {
		const fieldToDelete = collection.fields.find((f) => f._id === id);
		if (!fieldToDelete) return;

		const fieldName = fieldToDelete.name;
		const newFields = collection.fields.filter((f) => f._id !== id);

		// Remove deleted field from any composite indexes, and remove empty indexes
		const newIndexes = collection.indexes
			.map((idx) => ({
				...idx,
				fields: idx.fields.filter((f) => f !== fieldName)
			}))
			.filter((idx) => idx.fields.length > 0);

		onupdate({
			...collection,
			fields: newFields,
			indexes: newIndexes
		});
	}

	function addField() {
		onupdate({
			...collection,
			fields: [...collection.fields, createEmptyField()]
		});
	}

	function updateRules(rules: EditableRules) {
		onupdate({ ...collection, rules });
	}

	function updateIndexes(indexes: EditableIndex[]) {
		onupdate({ ...collection, indexes });
	}

	const hasRules = $derived(
		collection.rules.create ||
			collection.rules.read ||
			collection.rules.update ||
			collection.rules.delete
	);

	const hasPrimaryField = $derived(collection.fields.some((f) => f.primary));
</script>

<Card.Root>
	<Card.Header class="pb-3">
		<div class="flex items-center gap-3">
			<Input
				class="text-lg font-semibold h-10 max-w-xs"
				placeholder="Collection name"
				value={collection.name}
				onchange={(e) => updateName(e.currentTarget.value)}
				{disabled}
			/>

			<div class="ml-auto flex items-center gap-2">
				<Button
					variant="ghost"
					size="icon"
					class="h-8 w-8 text-destructive hover:text-destructive hover:bg-destructive/10"
					onclick={ondelete}
					{disabled}
				>
					<TrashIcon class="h-4 w-4" />
				</Button>
			</div>
		</div>
	</Card.Header>

	<Card.Content class="space-y-4">
		<div class="space-y-2">
			<div class="flex items-center justify-between">
				<h4 class="text-sm font-medium text-muted-foreground">Fields</h4>
				<Button variant="outline" size="sm" onclick={addField} {disabled}>
					<PlusIcon class="h-4 w-4 mr-2" />
					Add Field
				</Button>
			</div>

			<div class="space-y-2" role="list">
				{#each collection.fields as field (field._id)}
					<FieldEditor
						{field}
						{allCollections}
						{hasPrimaryField}
						onupdate={(f) => updateField(field._id, f)}
						ondelete={() => deleteField(field._id)}
						ondragstart={handleDragStart}
						ondragover={handleDragOver}
						ondragend={handleDragEnd}
						ondrop={handleDrop}
						isDragOver={dragOverFieldId === field._id}
						isDragging={draggingFieldId === field._id}
						{disabled}
					/>
				{/each}
			</div>
		</div>

		<Collapsible.Root bind:open={rulesOpen}>
			<Collapsible.Trigger class="flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors w-full py-2">
				{#if rulesOpen}
					<ChevronDownIcon class="h-4 w-4" />
				{:else}
					<ChevronRightIcon class="h-4 w-4" />
				{/if}
				<ShieldIcon class="h-4 w-4" />
				<span>Access Rules</span>
				{#if hasRules}
					<span class="ml-1 text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">
						Configured
					</span>
				{/if}
			</Collapsible.Trigger>
			<Collapsible.Content class="pt-2">
				<RulesEditor rules={collection.rules} onupdate={updateRules} {disabled} />
			</Collapsible.Content>
		</Collapsible.Root>

		<Collapsible.Root bind:open={indexesOpen}>
			<Collapsible.Trigger class="flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors w-full py-2">
				{#if indexesOpen}
					<ChevronDownIcon class="h-4 w-4" />
				{:else}
					<ChevronRightIcon class="h-4 w-4" />
				{/if}
				<DatabaseIcon class="h-4 w-4" />
				<span>Indexes</span>
				{#if collection.indexes.length > 0}
					<span class="ml-1 text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">
						{collection.indexes.length}
					</span>
				{/if}
			</Collapsible.Trigger>
			<Collapsible.Content class="pt-2">
				<IndexEditor
					indexes={collection.indexes}
					fields={collection.fields}
					onupdate={updateIndexes}
					{disabled}
				/>
			</Collapsible.Content>
		</Collapsible.Root>
	</Card.Content>
</Card.Root>

<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { Switch } from '$ui/switch';
	import { Badge } from '$ui/badge';
	import type { EditableIndex, EditableField } from './types';
	import TrashIcon from 'lucide-svelte/icons/trash-2';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import XIcon from 'lucide-svelte/icons/x';

	interface Props {
		indexes: EditableIndex[];
		fields: EditableField[];
		onupdate: (indexes: EditableIndex[]) => void;
		disabled?: boolean;
	}

	let { indexes, fields, onupdate, disabled = false }: Props = $props();

	function addIndex() {
		onupdate([
			...indexes,
			{
				_id: crypto.randomUUID(),
				name: '',
				fields: [],
				unique: false
			}
		]);
	}

	function updateIndex(id: string, updates: Partial<EditableIndex>) {
		onupdate(indexes.map((i) => (i._id === id ? { ...i, ...updates } : i)));
	}

	function removeIndex(id: string) {
		onupdate(indexes.filter((i) => i._id !== id));
	}

	function addFieldToIndex(indexId: string, fieldName: string) {
		const index = indexes.find((i) => i._id === indexId);
		if (index && !index.fields.includes(fieldName)) {
			updateIndex(indexId, { fields: [...index.fields, fieldName] });
		}
	}

	function removeFieldFromIndex(indexId: string, fieldName: string) {
		const index = indexes.find((i) => i._id === indexId);
		if (index) {
			updateIndex(indexId, { fields: index.fields.filter((f) => f !== fieldName) });
		}
	}

	const availableFields = $derived(fields.map((f) => f.name).filter((n) => n));
</script>

<div class="space-y-3">
	{#each indexes as index (index._id)}
		<div class="rounded-md border border-border p-3 space-y-3">
			<div class="flex items-center gap-3">
				<Input
					class="flex-1 h-8"
					placeholder="Index name"
					value={index.name}
					onchange={(e) => updateIndex(index._id, { name: e.currentTarget.value })}
					{disabled}
				/>

				<label class="flex items-center gap-1.5 text-sm">
					<Switch
						checked={index.unique ?? false}
						onCheckedChange={(v) => updateIndex(index._id, { unique: v })}
						disabled={disabled}
					/>
					<span class="text-muted-foreground">Unique</span>
				</label>

				<Button
					variant="ghost"
					size="icon"
					class="h-8 w-8 text-destructive hover:text-destructive hover:bg-destructive/10"
					onclick={() => removeIndex(index._id)}
					{disabled}
				>
					<TrashIcon class="h-4 w-4" />
				</Button>
			</div>

			<div class="flex flex-wrap items-center gap-2">
				<span class="text-sm text-muted-foreground">Fields:</span>
				{#each index.fields as fieldName}
					<Badge variant="secondary" class="gap-1">
						{fieldName}
						<button
							type="button"
							class="hover:text-destructive"
							onclick={() => removeFieldFromIndex(index._id, fieldName)}
							{disabled}
						>
							<XIcon class="h-3 w-3" />
						</button>
					</Badge>
				{/each}

				{#if availableFields.filter((f) => !index.fields.includes(f)).length > 0}
					<select
						class="h-7 rounded-md border border-input bg-background px-2 text-xs"
						value=""
						onchange={(e) => {
							if (e.currentTarget.value) {
								addFieldToIndex(index._id, e.currentTarget.value);
								e.currentTarget.value = '';
							}
						}}
						{disabled}
					>
						<option value="">+ Add field</option>
						{#each availableFields.filter((f) => !index.fields.includes(f)) as fieldName}
							<option value={fieldName}>{fieldName}</option>
						{/each}
					</select>
				{/if}
			</div>
		</div>
	{/each}

	<Button variant="outline" size="sm" onclick={addIndex} {disabled}>
		<PlusIcon class="h-4 w-4 mr-2" />
		Add Index
	</Button>
</div>

<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { Switch } from '$ui/switch';
	import { Label } from '$ui/label';
	import { Badge } from '$ui/badge';
	import FieldTypeSelect from './field-type-select.svelte';
	import {
		type EditableField,
		type EditableCollection,
		type FieldType,
		type SelectConfig,
		type RelationConfig,
		type SchemaValidationError,
		supportsValidation,
		VALIDATION_FORMATS,
		ON_DELETE_ACTIONS
	} from './types';
	import TrashIcon from 'lucide-svelte/icons/trash-2';
	import ChevronDownIcon from 'lucide-svelte/icons/chevron-down';
	import ChevronUpIcon from 'lucide-svelte/icons/chevron-up';
	import GripVerticalIcon from 'lucide-svelte/icons/grip-vertical';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import XIcon from 'lucide-svelte/icons/x';
	import AlertCircleIcon from 'lucide-svelte/icons/alert-circle';
	import { slide } from 'svelte/transition';

	interface Props {
		field: EditableField;
		allCollections: EditableCollection[];
		hasPrimaryField: boolean;
		onupdate: (field: EditableField) => void;
		ondelete: () => void;
		ondragstart?: (e: DragEvent, fieldId: string) => void;
		ondragover?: (e: DragEvent, fieldId: string) => void;
		ondragend?: () => void;
		ondrop?: (e: DragEvent, fieldId: string) => void;
		isDragOver?: boolean;
		isDragging?: boolean;
		disabled?: boolean;
		errors?: SchemaValidationError[];
	}

	let {
		field,
		allCollections,
		hasPrimaryField,
		onupdate,
		ondelete,
		ondragstart,
		ondragover,
		ondragend,
		ondrop,
		isDragOver = false,
		isDragging = false,
		disabled = false,
		errors = []
	}: Props = $props();

	const hasErrors = $derived(errors.length > 0);

	const primaryDisabled = $derived(disabled || (hasPrimaryField && !field.primary));

	let expanded = $state(false);
	let canDrag = $state(false);
	let newSelectValue = $state('');

	function updateField<K extends keyof EditableField>(key: K, value: EditableField[K]) {
		onupdate({ ...field, [key]: value });
	}

	function updateValidation<K extends keyof NonNullable<EditableField['validate']>>(
		key: K,
		value: NonNullable<EditableField['validate']>[K] | undefined
	) {
		const validate = { ...field.validate };
		if (value === undefined || value === '' || value === null) {
			delete validate[key];
		} else {
			validate[key] = value;
		}
		const hasValues = Object.keys(validate).length > 0;
		onupdate({ ...field, validate: hasValues ? validate : undefined });
	}

	function updateSelectConfig<K extends keyof SelectConfig>(key: K, value: SelectConfig[K]) {
		const select = { ...(field.select ?? { values: [] }) };
		select[key] = value;
		onupdate({ ...field, select });
	}

	function addSelectValue() {
		if (!newSelectValue.trim()) return;
		const values = [...(field.select?.values ?? []), newSelectValue.trim()];
		updateSelectConfig('values', values);
		newSelectValue = '';
	}

	function removeSelectValue(index: number) {
		const values = [...(field.select?.values ?? [])];
		values.splice(index, 1);
		updateSelectConfig('values', values);
	}

	function updateRelationConfig<K extends keyof RelationConfig>(key: K, value: RelationConfig[K]) {
		const relation = { ...(field.relation ?? { collection: '' }) };
		if (value === undefined || value === '') {
			delete relation[key];
		} else {
			relation[key] = value;
		}
		onupdate({ ...field, relation });
	}

	function handleTypeChange(type: FieldType) {
		const updated: EditableField = { ...field, type };
		if (type !== 'richtext') {
			delete updated.richtext;
		}
		if (type !== 'select') {
			delete updated.select;
		}
		if (type !== 'relation') {
			delete updated.relation;
		}
		if (!['string', 'text', 'int', 'float', 'email', 'url'].includes(type)) {
			delete updated.validate;
		}
		// Auto-expand for types that require config
		if (type === 'select' || type === 'relation') {
			expanded = true;
		}
		onupdate(updated);
	}

	const validationSupport = $derived(supportsValidation(field.type));

	const collectionOptions = $derived(
		allCollections.map((c) => ({ value: c.name, label: c.name }))
	);

	// Types that support default values in the options panel
	const supportsDefaultValue = $derived(
		['string', 'text', 'int', 'float', 'bool', 'datetime', 'date'].includes(field.type)
	);

	const hasAdvancedOptions = $derived(
		validationSupport.minLength ||
		validationSupport.maxLength ||
		validationSupport.min ||
		validationSupport.max ||
		validationSupport.format ||
		validationSupport.pattern ||
		field.type === 'richtext' ||
		field.type === 'select' ||
		field.type === 'relation' ||
		supportsDefaultValue
	);

	const relationFieldOptions = $derived.by(() => {
		if (!field.relation?.collection) return [];
		const col = allCollections.find((c) => c.name === field.relation?.collection);
		if (!col) return [];
		return col.fields
			.filter((f) => f.primary || f.unique)
			.map((f) => ({ value: f.name, label: f.name }));
	});

	const needsConfig = $derived(
		(field.type === 'select' && (!field.select?.values || field.select.values.length === 0)) ||
		(field.type === 'relation' && !field.relation?.collection)
	);
</script>

<div
	class="rounded-md border bg-card transition-colors
		{isDragOver ? 'border-primary border-2 bg-primary/5' : 'border-border'}
		{isDragging ? 'opacity-50' : ''}
		{hasErrors ? 'border-destructive' : needsConfig ? 'border-amber-500/50' : ''}"
	role="listitem"
	draggable={canDrag && !disabled}
	ondragstart={(e) => {
		if (disabled || !canDrag) {
			e.preventDefault();
			return;
		}
		e.dataTransfer!.effectAllowed = 'move';
		e.dataTransfer!.setData('text/plain', field._id);
		ondragstart?.(e, field._id);
	}}
	ondragend={() => {
		canDrag = false;
		ondragend?.();
	}}
	ondragover={(e) => {
		e.preventDefault();
		ondragover?.(e, field._id);
	}}
	ondrop={(e) => {
		e.preventDefault();
		ondrop?.(e, field._id);
	}}
>
	<!-- Main Row -->
	<div class="flex items-center gap-2 p-2">
		{#if hasErrors}
			<AlertCircleIcon class="h-4 w-4 text-destructive shrink-0" />
		{:else}
			<div
				role="button"
				tabindex="0"
				aria-label="Drag to reorder"
				class="cursor-grab text-muted-foreground hover:text-foreground active:cursor-grabbing select-none"
				onmousedown={() => (canDrag = true)}
				onmouseup={() => (canDrag = false)}
				onmouseleave={() => (canDrag = false)}
				onkeydown={(e) => {
					if (e.key === ' ' || e.key === 'Enter') {
						e.preventDefault();
						canDrag = !canDrag;
					}
				}}
			>
				<GripVerticalIcon class="h-4 w-4" />
			</div>
		{/if}

		<Input
			class="w-[180px] h-8 {hasErrors ? 'border-destructive' : ''}"
			placeholder="Field name"
			value={field.name}
			onchange={(e) => updateField('name', e.currentTarget.value)}
			{disabled}
		/>

		<FieldTypeSelect value={field.type} onchange={handleTypeChange} {disabled} />

		<div class="flex items-center gap-4 ml-2">
			<label class="flex items-center gap-1.5 text-sm">
				<Switch
					checked={field.primary ?? false}
					onCheckedChange={(v) => updateField('primary', v || undefined)}
					disabled={primaryDisabled}
				/>
				<span class="text-muted-foreground">Primary</span>
			</label>

			<label class="flex items-center gap-1.5 text-sm">
				<Switch
					checked={field.unique ?? false}
					onCheckedChange={(v) => updateField('unique', v || undefined)}
					disabled={disabled}
				/>
				<span class="text-muted-foreground">Unique</span>
			</label>

			<label class="flex items-center gap-1.5 text-sm">
				<Switch
					checked={field.nullable ?? false}
					onCheckedChange={(v) => updateField('nullable', v || undefined)}
					disabled={disabled}
				/>
				<span class="text-muted-foreground">Nullable</span>
			</label>

			<label class="flex items-center gap-1.5 text-sm">
				<Switch
					checked={field.index ?? false}
					onCheckedChange={(v) => updateField('index', v || undefined)}
					disabled={disabled}
				/>
				<span class="text-muted-foreground">Index</span>
			</label>
		</div>

		<div class="flex items-center gap-1 ml-auto">
			{#if hasAdvancedOptions}
				<Button
					variant={expanded ? 'secondary' : 'ghost'}
					size="sm"
					class="h-8 gap-1 {needsConfig ? 'text-amber-600' : ''}"
					onclick={() => (expanded = !expanded)}
					{disabled}
				>
					{#if expanded}
						<ChevronUpIcon class="h-4 w-4" />
					{:else}
						<ChevronDownIcon class="h-4 w-4" />
					{/if}
					Options
					{#if needsConfig}
						<Badge variant="outline" class="ml-1 text-xs text-amber-600 border-amber-500">Required</Badge>
					{/if}
				</Button>
			{/if}

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

	{#if hasErrors}
		<div class="px-4 pb-2 pt-0">
			{#each errors as error}
				<p class="text-sm text-destructive">{error.message}</p>
			{/each}
		</div>
	{/if}

	<!-- Expanded Options Panel -->
	{#if expanded && hasAdvancedOptions}
		<div
			class="border-t border-border bg-muted/30 px-4 py-3"
			transition:slide={{ duration: 150 }}
		>
			<div class="grid gap-4">
				<!-- Select Field Options -->
				{#if field.type === 'select'}
					<div class="space-y-3">
						<div class="flex items-center justify-between">
							<Label class="text-sm font-medium">Select Options</Label>
							<label class="flex items-center gap-2 text-sm">
								<span class="text-muted-foreground">Max selections:</span>
								<Input
									type="number"
									min="0"
									class="w-20 h-7"
									placeholder="∞"
									value={field.select?.maxSelect ?? ''}
									onchange={(e) => {
										const v = e.currentTarget.value;
										updateSelectConfig('maxSelect', v ? parseInt(v) : undefined);
									}}
								/>
							</label>
						</div>
						
						<div class="flex flex-wrap gap-2">
							{#each field.select?.values ?? [] as value, i}
								<Badge variant="secondary" class="gap-1 pr-1 text-sm">
									{value}
									<button
										type="button"
										class="ml-1 hover:bg-muted rounded-sm p-0.5"
										onclick={() => removeSelectValue(i)}
									>
										<XIcon class="h-3 w-3" />
									</button>
								</Badge>
							{/each}
						</div>

						<div class="flex gap-2">
							<Input
								class="flex-1 h-8"
								placeholder="Add option value..."
								bind:value={newSelectValue}
								onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addSelectValue())}
							/>
							<Button size="sm" variant="outline" class="h-8" onclick={addSelectValue}>
								<PlusIcon class="h-4 w-4 mr-1" />
								Add
							</Button>
						</div>

						{#if !field.select?.values?.length}
							<p class="text-sm text-amber-600">At least one option value is required.</p>
						{/if}
					</div>
				{/if}

				<!-- Relation Field Options -->
				{#if field.type === 'relation'}
					<div class="space-y-3">
						<Label class="text-sm font-medium">Relation Configuration</Label>
						
						<div class="grid grid-cols-3 gap-3">
							<div class="space-y-1">
								<Label class="text-xs text-muted-foreground">Collection</Label>
								<select
									class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
									value={field.relation?.collection ?? ''}
									onchange={(e) => updateRelationConfig('collection', e.currentTarget.value)}
								>
									<option value="">Select collection...</option>
									{#each collectionOptions as opt}
										<option value={opt.value}>{opt.label}</option>
									{/each}
								</select>
							</div>

							<div class="space-y-1">
								<Label class="text-xs text-muted-foreground">Field</Label>
								<select
									class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
									value={field.relation?.field ?? 'id'}
									onchange={(e) => updateRelationConfig('field', e.currentTarget.value)}
									disabled={!field.relation?.collection}
								>
									<option value="id">id (default)</option>
									{#each relationFieldOptions as opt}
										{#if opt.value !== 'id'}
											<option value={opt.value}>{opt.label}</option>
										{/if}
									{/each}
								</select>
							</div>

							<div class="space-y-1">
								<Label class="text-xs text-muted-foreground">On Delete</Label>
								<select
									class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
									value={field.relation?.onDelete ?? 'restrict'}
									onchange={(e) => updateRelationConfig('onDelete', e.currentTarget.value as RelationConfig['onDelete'])}
								>
									{#each ON_DELETE_ACTIONS as action}
										<option value={action}>{action}</option>
									{/each}
								</select>
							</div>
						</div>

						{#if !field.relation?.collection}
							<p class="text-sm text-amber-600">A target collection must be selected.</p>
						{/if}
					</div>
				{/if}

				<!-- Default Value (for types that support it) -->
				{#if supportsDefaultValue}
					<div class="space-y-1">
						<Label class="text-xs text-muted-foreground">Default Value</Label>
						<Input
							class="h-8 max-w-xs"
							placeholder="auto, now, or literal"
							value={field.default ?? ''}
							onchange={(e) => updateField('default', e.currentTarget.value || undefined)}
						/>
					</div>
				{/if}

				<!-- Validation Options -->
				{#if validationSupport.minLength || validationSupport.maxLength || validationSupport.min || validationSupport.max || validationSupport.format || validationSupport.pattern}
					<div>
						<Label class="text-sm font-medium mb-3 block">Validation</Label>
						<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
							{#if validationSupport.minLength}
								<div class="space-y-1">
									<Label class="text-xs text-muted-foreground">Min Length</Label>
									<Input
										type="number"
										class="h-8"
										min="0"
										value={field.validate?.minLength ?? ''}
										onchange={(e) => {
											const v = e.currentTarget.value;
											updateValidation('minLength', v ? parseInt(v) : undefined);
										}}
									/>
								</div>
							{/if}
							
							{#if validationSupport.maxLength}
								<div class="space-y-1">
									<Label class="text-xs text-muted-foreground">Max Length</Label>
									<Input
										type="number"
										class="h-8"
										min="0"
										value={field.validate?.maxLength ?? ''}
										onchange={(e) => {
											const v = e.currentTarget.value;
											updateValidation('maxLength', v ? parseInt(v) : undefined);
										}}
									/>
								</div>
							{/if}

							{#if validationSupport.min}
								<div class="space-y-1">
									<Label class="text-xs text-muted-foreground">Min Value</Label>
									<Input
										type="number"
										class="h-8"
										value={field.validate?.min ?? ''}
										onchange={(e) => {
											const v = e.currentTarget.value;
											updateValidation('min', v ? parseFloat(v) : undefined);
										}}
									/>
								</div>
							{/if}

							{#if validationSupport.max}
								<div class="space-y-1">
									<Label class="text-xs text-muted-foreground">Max Value</Label>
									<Input
										type="number"
										class="h-8"
										value={field.validate?.max ?? ''}
										onchange={(e) => {
											const v = e.currentTarget.value;
											updateValidation('max', v ? parseFloat(v) : undefined);
										}}
									/>
								</div>
							{/if}

							{#if validationSupport.format}
								<div class="space-y-1">
									<Label class="text-xs text-muted-foreground">Format</Label>
									<select
										class="w-full h-8 rounded-md border border-input bg-background px-2 text-sm"
										value={field.validate?.format ?? ''}
										onchange={(e) =>
											updateValidation(
												'format',
												(e.currentTarget.value as NonNullable<EditableField['validate']>['format']) || undefined
											)}
									>
										<option value="">None</option>
										{#each VALIDATION_FORMATS as format}
											<option value={format}>{format}</option>
										{/each}
									</select>
								</div>
							{/if}

							{#if validationSupport.pattern}
								<div class="space-y-1 col-span-2">
									<Label class="text-xs text-muted-foreground">Pattern (regex)</Label>
									<Input
										class="h-8 font-mono text-sm"
										placeholder="^[a-z]+$"
										value={field.validate?.pattern ?? ''}
										onchange={(e) => updateValidation('pattern', e.currentTarget.value || undefined)}
									/>
								</div>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

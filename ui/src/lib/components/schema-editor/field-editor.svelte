<script lang="ts">
	import { Input } from '$ui/input';
	import { Button } from '$ui/button';
	import { Switch } from '$ui/switch';
	import { Label } from '$ui/label';
	import * as Popover from '$ui/popover';
	import FieldTypeSelect from './field-type-select.svelte';
	import {
		type EditableField,
		type EditableCollection,
		type FieldType,
		supportsValidation,
		VALIDATION_FORMATS,
		ON_DELETE_ACTIONS
	} from './types';
	import TrashIcon from 'lucide-svelte/icons/trash-2';
	import SettingsIcon from 'lucide-svelte/icons/settings';
	import GripVerticalIcon from 'lucide-svelte/icons/grip-vertical';

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
		disabled = false
	}: Props = $props();

	const primaryDisabled = $derived(disabled || (hasPrimaryField && !field.primary));

	let optionsOpen = $state(false);
	let rowEl: HTMLDivElement | undefined = $state();
	let canDrag = $state(false);

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

	function handleTypeChange(type: FieldType) {
		const updated: EditableField = { ...field, type };
		if (type !== 'richtext') {
			delete updated.richtext;
		}
		if (!['string', 'text', 'int', 'float'].includes(type)) {
			delete updated.validate;
		}
		onupdate(updated);
	}

	const validationSupport = $derived(supportsValidation(field.type));
	const hasAdvancedOptions = $derived(
		validationSupport.minLength ||
			validationSupport.maxLength ||
			validationSupport.min ||
			validationSupport.max ||
			validationSupport.format ||
			validationSupport.pattern ||
			field.type === 'richtext'
	);

	const referenceOptions = $derived(
		allCollections.flatMap((c) =>
			c.fields
				.filter((f) => f.primary || f.unique)
				.map((f) => ({ value: `${c.name}.${f.name}`, label: `${c.name}.${f.name}` }))
		)
	);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	bind:this={rowEl}
	class="flex items-center gap-2 rounded-md border p-2 bg-card transition-colors group
		{isDragOver ? 'border-primary border-2 bg-primary/5' : 'border-border hover:bg-accent/50'}
		{isDragging ? 'opacity-50' : ''}"
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
	<div
		class="cursor-grab text-muted-foreground hover:text-foreground active:cursor-grabbing select-none"
		onmousedown={() => (canDrag = true)}
		onmouseup={() => (canDrag = false)}
		onmouseleave={() => (canDrag = false)}
	>
		<GripVerticalIcon class="h-4 w-4" />
	</div>

	<Input
		class="w-[180px] h-8"
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
		{#if hasAdvancedOptions || referenceOptions.length > 0}
			<Popover.Root bind:open={optionsOpen}>
				<Popover.Trigger>
					<Button variant="ghost" size="icon" class="h-8 w-8" {disabled}>
						<SettingsIcon class="h-4 w-4" />
					</Button>
				</Popover.Trigger>
				<Popover.Content class="w-80">
					<div class="space-y-4">
						<h4 class="font-medium">Field Options</h4>

						<div class="space-y-3">
							<div class="space-y-1">
								<Label>Default Value</Label>
								<Input
									placeholder="e.g., auto, now, or literal"
									value={field.default ?? ''}
									onchange={(e) =>
										updateField('default', e.currentTarget.value || undefined)}
								/>
							</div>

							{#if referenceOptions.length > 0}
								<div class="space-y-1">
									<Label>References</Label>
									<select
										class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
										value={field.references ?? ''}
										onchange={(e) =>
											updateField('references', e.currentTarget.value || undefined)}
									>
										<option value="">None</option>
										{#each referenceOptions as opt}
											<option value={opt.value}>{opt.label}</option>
										{/each}
									</select>
								</div>

								{#if field.references}
									<div class="space-y-1">
										<Label>On Delete</Label>
										<select
											class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
											value={field.onDelete ?? 'restrict'}
											onchange={(e) =>
												updateField(
													'onDelete',
													(e.currentTarget.value as EditableField['onDelete']) || undefined
												)}
										>
											{#each ON_DELETE_ACTIONS as action}
												<option value={action}>{action}</option>
											{/each}
										</select>
									</div>
								{/if}
							{/if}

							{#if validationSupport.minLength || validationSupport.maxLength}
								<div class="grid grid-cols-2 gap-2">
									{#if validationSupport.minLength}
										<div class="space-y-1">
											<Label>Min Length</Label>
											<Input
												type="number"
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
											<Label>Max Length</Label>
											<Input
												type="number"
												min="0"
												value={field.validate?.maxLength ?? ''}
												onchange={(e) => {
													const v = e.currentTarget.value;
													updateValidation('maxLength', v ? parseInt(v) : undefined);
												}}
											/>
										</div>
									{/if}
								</div>
							{/if}

							{#if validationSupport.min || validationSupport.max}
								<div class="grid grid-cols-2 gap-2">
									{#if validationSupport.min}
										<div class="space-y-1">
											<Label>Min Value</Label>
											<Input
												type="number"
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
											<Label>Max Value</Label>
											<Input
												type="number"
												value={field.validate?.max ?? ''}
												onchange={(e) => {
													const v = e.currentTarget.value;
													updateValidation('max', v ? parseFloat(v) : undefined);
												}}
											/>
										</div>
									{/if}
								</div>
							{/if}

							{#if validationSupport.format}
								<div class="space-y-1">
									<Label>Format</Label>
									<select
										class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
										value={field.validate?.format ?? ''}
										onchange={(e) =>
											updateValidation(
												'format',
												(e.currentTarget.value as NonNullable<EditableField['validate']>['format']) ||
													undefined
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
								<div class="space-y-1">
									<Label>Pattern (regex)</Label>
									<Input
										placeholder="^[a-z]+$"
										value={field.validate?.pattern ?? ''}
										onchange={(e) =>
											updateValidation('pattern', e.currentTarget.value || undefined)}
									/>
								</div>
							{/if}
						</div>
					</div>
				</Popover.Content>
			</Popover.Root>
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

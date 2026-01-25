<script lang="ts">
	import * as Select from '$ui/select';
	import { FIELD_TYPES, getFieldTypeInfo, type FieldType } from './types';

	interface Props {
		value: FieldType;
		onchange: (value: FieldType) => void;
		disabled?: boolean;
	}

	let { value, onchange, disabled = false }: Props = $props();

	function handleChange(v: string | undefined) {
		if (v && FIELD_TYPES.includes(v as FieldType)) {
			onchange(v as FieldType);
		}
	}
</script>

<Select.Root type="single" {value} onValueChange={handleChange}>
	<Select.Trigger class="w-[140px]" {disabled}>
		{getFieldTypeInfo(value).label}
	</Select.Trigger>
	<Select.Content>
		{#each FIELD_TYPES as type}
			{@const info = getFieldTypeInfo(type)}
			<Select.Item value={type} label={info.label}>
				<div class="flex flex-col">
					<span>{info.label}</span>
					<span class="text-xs text-muted-foreground">{info.description}</span>
				</div>
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>

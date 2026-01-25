<script lang="ts">
	import CelInput from './cel-input.svelte';
	import type { EditableRules } from './types';

	interface Props {
		rules: EditableRules;
		onupdate: (rules: EditableRules) => void;
		disabled?: boolean;
		fieldNames?: string[];
	}

	let { rules, onupdate, disabled = false, fieldNames = [] }: Props = $props();

	function updateRule(key: keyof EditableRules, value: string) {
		const updated = { ...rules };
		if (value) {
			updated[key] = value;
		} else {
			delete updated[key];
		}
		onupdate(updated);
	}
</script>

<div class="space-y-4">
	<CelInput
		label="Create Rule"
		placeholder='e.g., auth.id != "" (require auth)'
		value={rules.create ?? ''}
		onchange={(v) => updateRule('create', v)}
		{disabled}
		{fieldNames}
	/>

	<CelInput
		label="Read Rule"
		placeholder="e.g., true (public access)"
		value={rules.read ?? ''}
		onchange={(v) => updateRule('read', v)}
		{disabled}
		{fieldNames}
	/>

	<CelInput
		label="Update Rule"
		placeholder="e.g., auth.id == doc.owner_id"
		value={rules.update ?? ''}
		onchange={(v) => updateRule('update', v)}
		{disabled}
		{fieldNames}
	/>

	<CelInput
		label="Delete Rule"
		placeholder='e.g., auth.role == "admin"'
		value={rules.delete ?? ''}
		onchange={(v) => updateRule('delete', v)}
		{disabled}
		{fieldNames}
	/>
</div>

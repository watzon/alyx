<script lang="ts">
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
	import type { EditableRules } from './types';

	interface Props {
		rules: EditableRules;
		onupdate: (rules: EditableRules) => void;
		disabled?: boolean;
	}

	let { rules, onupdate, disabled = false }: Props = $props();

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

<div class="space-y-3">
	<div class="space-y-1">
		<Label class="text-xs text-muted-foreground">Create Rule (CEL expression)</Label>
		<Input
			placeholder='e.g., auth.isAdmin || auth.userId == doc.owner_id'
			value={rules.create ?? ''}
			onchange={(e) => updateRule('create', e.currentTarget.value)}
			{disabled}
		/>
	</div>

	<div class="space-y-1">
		<Label class="text-xs text-muted-foreground">Read Rule</Label>
		<Input
			placeholder='e.g., true (public) or auth.userId != ""'
			value={rules.read ?? ''}
			onchange={(e) => updateRule('read', e.currentTarget.value)}
			{disabled}
		/>
	</div>

	<div class="space-y-1">
		<Label class="text-xs text-muted-foreground">Update Rule</Label>
		<Input
			placeholder="e.g., auth.userId == doc.owner_id"
			value={rules.update ?? ''}
			onchange={(e) => updateRule('update', e.currentTarget.value)}
			{disabled}
		/>
	</div>

	<div class="space-y-1">
		<Label class="text-xs text-muted-foreground">Delete Rule</Label>
		<Input
			placeholder="e.g., auth.isAdmin"
			value={rules.delete ?? ''}
			onchange={(e) => updateRule('delete', e.currentTarget.value)}
			{disabled}
		/>
	</div>
</div>

<script lang="ts">
	import { Input } from '$ui/input';
	import * as Select from '$ui/select';
	import SearchIcon from '@lucide/svelte/icons/search';

	interface Props {
		search: string;
		mimeType: string;
		onSearchChange: (value: string) => void;
		onMimeTypeChange: (value: string) => void;
	}

	let { search, mimeType, onSearchChange, onMimeTypeChange }: Props = $props();

	const mimeOptions = [
		{ value: '', label: 'All Files' },
		{ value: 'image/', label: 'Images' },
		{ value: 'video/', label: 'Videos' },
		{ value: 'application/pdf', label: 'Documents' },
		{ value: 'audio/', label: 'Audio' },
		{ value: 'other', label: 'Other' }
	];

	const selectedLabel = $derived(
		mimeOptions.find((o) => o.value === mimeType)?.label ?? 'All Files'
	);

	function handleMimeChange(v: string | undefined) {
		if (v !== undefined) {
			onMimeTypeChange(v);
		}
	}
</script>

<div class="flex items-center gap-4">
	<div class="relative flex-1 max-w-sm">
		<SearchIcon class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
		<Input
			type="search"
			placeholder="Search files..."
			class="pl-9"
			value={search}
			oninput={(e) => onSearchChange(e.currentTarget.value)}
		/>
	</div>

	<Select.Root type="single" value={mimeType} onValueChange={handleMimeChange}>
		<Select.Trigger class="w-[180px]">
			{selectedLabel}
		</Select.Trigger>
		<Select.Content>
			{#each mimeOptions as option}
				<Select.Item value={option.value} label={option.label}>
					{option.label}
				</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
</div>

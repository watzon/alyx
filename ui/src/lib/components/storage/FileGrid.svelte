<script lang="ts">
	import type { FileMetadata } from '$lib/api/client';
	import FileCard from './FileCard.svelte';
	import * as Card from '$ui/card';
	import FileIcon from '@lucide/svelte/icons/file';

	interface Props {
		bucket: string;
		files: FileMetadata[];
		selectedIds: string[];
		onSelectionChange: (ids: string[]) => void;
		onDelete?: (ids: string[]) => void;
	}

	let { bucket, files, selectedIds, onSelectionChange, onDelete }: Props = $props();

	function toggleSelection(id: string) {
		if (selectedIds.includes(id)) {
			onSelectionChange(selectedIds.filter((i) => i !== id));
		} else {
			onSelectionChange([...selectedIds, id]);
		}
	}

	function handleDelete(id: string) {
		onDelete?.([id]);
	}
</script>

{#if files.length === 0}
	<Card.Root>
		<Card.Content class="py-10 text-center">
			<FileIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
			<h3 class="mt-3 text-sm font-medium">No files</h3>
			<p class="mt-1 text-sm text-muted-foreground">Upload files to get started</p>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="grid gap-4 grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
		{#each files as file (file.id)}
			<FileCard
				{bucket}
				{file}
				selected={selectedIds.includes(file.id)}
				onToggle={() => toggleSelection(file.id)}
				onDelete={onDelete ? () => handleDelete(file.id) : undefined}
			/>
		{/each}
	</div>
{/if}

<script lang="ts">
	import type { FileMetadata } from '$lib/api/client';
	import FileTable from './FileTable.svelte';
	import FileGrid from './FileGrid.svelte';

	interface Props {
		bucket: string;
		files: FileMetadata[];
		viewMode: 'table' | 'grid';
		selectedIds: string[];
		onSelectionChange: (ids: string[]) => void;
		onDelete?: (ids: string[]) => void;
		onPreview?: (file: FileMetadata) => void;
	}

	let { bucket, files, viewMode, selectedIds, onSelectionChange, onDelete, onPreview }: Props = $props();
</script>

{#if viewMode === 'table'}
	<FileTable {bucket} {files} {selectedIds} {onSelectionChange} {onDelete} {onPreview} />
{:else}
	<FileGrid {bucket} {files} {selectedIds} {onSelectionChange} {onDelete} {onPreview} />
{/if}

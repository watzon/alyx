<script lang="ts">
	import type { FileMetadata } from '$lib/api/client';
	import { files } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Button } from '$ui/button';
	import * as Tooltip from '$ui/tooltip';
	import FileIcon from '@lucide/svelte/icons/file';
	import ImageIcon from '@lucide/svelte/icons/image';
	import TrashIcon from '@lucide/svelte/icons/trash-2';

	interface Props {
		bucket: string;
		file: FileMetadata;
		selected: boolean;
		onToggle: () => void;
		onDelete?: () => void;
		onPreview?: () => void;
	}

	let { bucket, file, selected, onToggle, onDelete, onPreview }: Props = $props();

	let isImage = $derived(file.mime_type.startsWith('image/'));
	let viewUrl = $derived(isImage ? files.getViewUrl(bucket, file.id) : null);
	let FileIconComponent = $derived(getFileIcon(file.mime_type));

	function formatSize(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
	}

	function getFileIcon(mimeType: string) {
		if (mimeType.startsWith('image/')) return ImageIcon;
		return FileIcon;
	}
</script>

<Card.Root class="relative group overflow-hidden p-0">
	<div class="absolute top-2 left-2 z-10">
		<label class="flex items-center justify-center w-5 h-5 rounded border bg-background/80 backdrop-blur-sm cursor-pointer transition-colors {selected ? 'border-primary bg-primary' : 'border-border'}">
			<input type="checkbox" checked={selected} onchange={onToggle} class="sr-only" />
			{#if selected}
				<svg class="w-3.5 h-3.5 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
				</svg>
			{/if}
		</label>
	</div>
	{#if onDelete}
		<div class="absolute top-2 right-2 z-10 opacity-0 group-hover:opacity-100 transition-opacity">
			<Button variant="ghost" size="icon" class="h-7 w-7 bg-background/80 backdrop-blur-sm" onclick={onDelete}>
				<TrashIcon class="h-3.5 w-3.5 text-destructive" />
			</Button>
		</div>
	{/if}
	<div class="p-0">
		{#if viewUrl}
			<button class="w-full h-48 block" onclick={onPreview}>
				<img src={viewUrl} alt={file.name} class="w-full h-48 object-cover rounded-t-lg bg-muted" loading="lazy" />
			</button>
		{:else}
			<button class="w-full h-48 bg-muted flex flex-col items-center justify-center rounded-t-lg" onclick={onPreview}>
				<FileIconComponent class="h-16 w-16 text-muted-foreground/50 mb-2" />
				<span class="text-xs text-muted-foreground uppercase tracking-wide">{file.mime_type.split('/')[1] || 'file'}</span>
			</button>
		{/if}
		<div class="p-4">
			<Tooltip.Provider>
				<Tooltip.Root>
					<Tooltip.Trigger class="block w-full text-left">
						<p class="text-sm font-medium truncate" title={file.name}>{file.name}</p>
					</Tooltip.Trigger>
					<Tooltip.Content>
						<p class="max-w-xs">{file.name}</p>
					</Tooltip.Content>
				</Tooltip.Root>
			</Tooltip.Provider>
			<div class="flex items-center justify-between mt-2">
				<p class="text-xs text-muted-foreground">{formatSize(file.size)}</p>
				<p class="text-xs text-muted-foreground">{new Date(file.created_at).toLocaleDateString()}</p>
			</div>
		</div>
	</div>
</Card.Root>

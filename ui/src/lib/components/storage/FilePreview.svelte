<script lang="ts">
	import { files, type FileMetadata } from '$lib/api/client';
	import * as Dialog from '$ui/dialog';
	import { Button } from '$ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import FileIcon from '@lucide/svelte/icons/file';
	import ImageIcon from '@lucide/svelte/icons/image';
	import VideoIcon from '@lucide/svelte/icons/video';

	interface Props {
		file: FileMetadata | null;
		open: boolean;
		bucket: string;
		onClose: () => void;
	}

	let { file, open, bucket, onClose }: Props = $props();

	const isImage = $derived(file?.mime_type.startsWith('image/') ?? false);
	const isVideo = $derived(file?.mime_type.startsWith('video/') ?? false);
	const viewUrl = $derived(file ? files.getViewUrl(bucket, file.id) : null);
	const downloadUrl = $derived(file ? files.getDownloadUrl(bucket, file.id) : null);

	function formatSize(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
	}

	function getFileIcon(mimeType: string) {
		if (mimeType.startsWith('image/')) return ImageIcon;
		if (mimeType.startsWith('video/')) return VideoIcon;
		return FileIcon;
	}

	function handleOpenChange(o: boolean) {
		if (!o) onClose();
	}
</script>

<Dialog.Root {open} onOpenChange={handleOpenChange}>
	<Dialog.Content class="max-w-4xl max-h-[90vh] overflow-y-auto">
		{#if file}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-2">
					{@const FileIconComponent = getFileIcon(file.mime_type)}
					<FileIconComponent class="h-5 w-5 text-muted-foreground" />
					<span class="truncate">{file.name}</span>
				</Dialog.Title>
				<Dialog.Description>
					{formatSize(file.size)} • {file.mime_type} • {new Date(file.created_at).toLocaleDateString()}
				</Dialog.Description>
			</Dialog.Header>

			<div class="mt-4 flex items-center justify-center bg-muted/50 rounded-lg min-h-[300px]">
				{#if isImage && viewUrl}
					<img src={viewUrl} alt={file.name} class="max-w-full max-h-[60vh] h-auto rounded-lg object-contain" />
				{:else if isVideo && viewUrl}
					<video src={viewUrl} controls class="max-w-full max-h-[60vh] h-auto rounded-lg">
						<track kind="captions" src="" label="No captions" />
					</video>
				{:else}
					{@const FileIconComponent = getFileIcon(file.mime_type)}
					<div class="flex flex-col items-center justify-center py-16">
						<FileIconComponent class="h-24 w-24 text-muted-foreground/30 mb-4" />
						<p class="text-muted-foreground">Preview not available for this file type</p>
					</div>
				{/if}
			</div>

			<Dialog.Footer class="mt-4 gap-2">
				<Button variant="outline" onclick={onClose}>Close</Button>
				{#if downloadUrl}
					<Button onclick={() => window.open(downloadUrl, '_blank')}>
						<DownloadIcon class="mr-2 h-4 w-4" />
						Download
					</Button>
				{/if}
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>

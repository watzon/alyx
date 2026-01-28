<script lang="ts">
	import type { FileMetadata } from '$lib/api/client';
	import { files as filesApi } from '$lib/api/client';
	import { toast } from 'svelte-sonner';
	import * as Table from '$ui/table';
	import { Button } from '$ui/button';
	import * as Card from '$ui/card';
	import * as Tooltip from '$ui/tooltip';
	import FileIcon from '@lucide/svelte/icons/file';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import LinkIcon from '@lucide/svelte/icons/link';

	interface Props {
		bucket: string;
		files: FileMetadata[];
		selectedIds: string[];
		onSelectionChange: (ids: string[]) => void;
		onDelete?: (ids: string[]) => void;
		onPreview?: (file: FileMetadata) => void;
	}

	let { bucket, files, selectedIds, onSelectionChange, onDelete, onPreview }: Props = $props();

	function toggleSelection(id: string) {
		if (selectedIds.includes(id)) {
			onSelectionChange(selectedIds.filter((i) => i !== id));
		} else {
			onSelectionChange([...selectedIds, id]);
		}
	}

	function toggleAll() {
		if (selectedIds.length === files.length) {
			onSelectionChange([]);
		} else {
			onSelectionChange(files.map((f) => f.id));
		}
	}

	function formatSize(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
	}

	function getFileIcon(mimeType: string) {
		// Could be extended to return different icons based on mime type
		return FileIcon;
	}

	let isGenerating = $state<string | null>(null);

	async function handleCopySignedUrl(file: FileMetadata) {
		isGenerating = file.id;
		try {
			const result = await filesApi.generateSignedUrl(bucket, file.id);
			if (result.error) throw new Error(result.error.message);
			await navigator.clipboard.writeText(result.data!.url);
			toast.success('Signed URL copied to clipboard');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to generate signed URL');
		} finally {
			isGenerating = null;
		}
	}
</script>

{#if !files || files.length === 0}
	<Card.Root>
		<Card.Content class="py-10 text-center">
			<FileIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
			<h3 class="mt-3 text-sm font-medium">No files</h3>
			<p class="mt-1 text-sm text-muted-foreground">Upload files to get started</p>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="rounded-md border overflow-hidden">
		<Table.Root>
			<Table.Header class="sticky top-0 z-20 bg-card">
				<Table.Row>
					<Table.Head class="w-12">
						<label class="flex items-center justify-center w-4 h-4 rounded border cursor-pointer transition-colors {selectedIds.length === files.length ? 'border-primary bg-primary' : 'border-border bg-background'}">
							<input type="checkbox" checked={selectedIds.length === files.length} indeterminate={selectedIds.length > 0 && selectedIds.length < files.length} onchange={toggleAll} class="sr-only" />
							{#if selectedIds.length === files.length}
								<svg class="w-3 h-3 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
								</svg>
							{:else if selectedIds.length > 0}
								<svg class="w-3 h-3 text-primary" fill="currentColor" viewBox="0 0 24 24">
									<rect x="6" y="11" width="12" height="2" rx="1" />
								</svg>
							{/if}
						</label>
					</Table.Head>
					<Table.Head class="w-12">Preview</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head class="w-24">Size</Table.Head>
					<Table.Head class="w-32">Type</Table.Head>
					<Table.Head class="w-32">Date</Table.Head>
					<Table.Head class="w-16">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each files as file (file.id)}
					<Table.Row class="hover:bg-muted/30 transition-colors">
						<Table.Cell>
							<label class="flex items-center justify-center w-4 h-4 rounded border cursor-pointer transition-colors {selectedIds.includes(file.id) ? 'border-primary bg-primary' : 'border-border bg-background'}">
								<input type="checkbox" checked={selectedIds.includes(file.id)} onchange={() => toggleSelection(file.id)} class="sr-only" />
								{#if selectedIds.includes(file.id)}
									<svg class="w-3 h-3 text-primary-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
									</svg>
								{/if}
							</label>
						</Table.Cell>
						<Table.Cell>
							{#if file.mime_type.startsWith('image/')}
								<button onclick={() => onPreview?.(file)}>
									<img src={`/api/files/${bucket}/${file.id}/view`} alt="" class="w-8 h-8 object-cover rounded" />
								</button>
							{:else}
								<button class="w-8 h-8 rounded bg-muted flex items-center justify-center" onclick={() => onPreview?.(file)}>
									<FileIcon class="w-4 h-4 text-muted-foreground" />
								</button>
							{/if}
						</Table.Cell>
						<Table.Cell>
							<Tooltip.Provider>
								<Tooltip.Root>
									<Tooltip.Trigger class="text-left">
										<button class="font-medium truncate block max-w-[200px] text-left" onclick={() => onPreview?.(file)}>
											{file.name}
										</button>
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="max-w-xs">{file.name}</p>
									</Tooltip.Content>
								</Tooltip.Root>
							</Tooltip.Provider>
						</Table.Cell>
						<Table.Cell class="text-muted-foreground">{formatSize(file.size)}</Table.Cell>
						<Table.Cell class="text-muted-foreground text-xs">{file.mime_type}</Table.Cell>
						<Table.Cell class="text-muted-foreground text-xs">{new Date(file.created_at).toLocaleDateString()}</Table.Cell>
						<Table.Cell>
							<div class="flex items-center gap-1">
								<Tooltip.Provider>
									<Tooltip.Root>
										<Tooltip.Trigger>
											<Button variant="ghost" size="icon" class="h-8 w-8" onclick={() => handleCopySignedUrl(file)} disabled={isGenerating === file.id}>
												<LinkIcon class="h-4 w-4" />
											</Button>
										</Tooltip.Trigger>
										<Tooltip.Content>Copy signed URL</Tooltip.Content>
									</Tooltip.Root>
								</Tooltip.Provider>

								{#if onDelete}
									<Tooltip.Provider>
										<Tooltip.Root>
											<Tooltip.Trigger>
												<Button variant="ghost" size="icon" class="h-8 w-8" onclick={() => onDelete([file.id])}>
													<TrashIcon class="h-4 w-4 text-destructive" />
												</Button>
											</Tooltip.Trigger>
											<Tooltip.Content>Delete file</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
								{/if}
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

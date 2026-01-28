<script lang="ts">
	import { files, type FileMetadata } from '$lib/api/client';
	import { Button } from '$ui/button';
	import * as Card from '$ui/card';
	import { toast } from 'svelte-sonner';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import LoaderIcon from '@lucide/svelte/icons/loader-circle';

	let { bucket, onUploadComplete } = $props<{
		bucket: string;
		onUploadComplete: (file: FileMetadata) => void;
	}>();

	let isDragging = $state(false);
	let uploading = $state(false);
	let uploadProgress = $state<Map<string, number>>(new Map());
	let fileInput: HTMLInputElement;

	function handleDragEnter(e: DragEvent) {
		e.preventDefault();
		isDragging = true;
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
	}

	function handleDragLeave(e: DragEvent) {
		e.preventDefault();
		isDragging = false;
	}

	async function handleDrop(e: DragEvent) {
		e.preventDefault();
		isDragging = false;

		const droppedFiles = Array.from(e.dataTransfer?.files || []);
		await uploadFiles(droppedFiles);
	}

	function handleFileSelect(e: Event) {
		const target = e.target as HTMLInputElement;
		const selectedFiles = Array.from(target.files || []);
		uploadFiles(selectedFiles);
		target.value = ''; // Reset input
	}

	async function uploadFiles(fileList: File[]) {
		if (fileList.length === 0) return;

		uploading = true;

		for (const file of fileList) {
			try {
				uploadProgress.set(file.name, 0);
				uploadProgress = uploadProgress;

				const result = await files.upload(bucket, file);

				if (result.error) {
					toast.error(`Failed to upload ${file.name}: ${result.error.message}`);
				} else {
					uploadProgress.set(file.name, 100);
					uploadProgress = uploadProgress;
					toast.success(`Uploaded ${file.name}`);
					onUploadComplete(result.data!);
				}
			} catch (err) {
				toast.error(`Error uploading ${file.name}`);
			} finally {
				setTimeout(() => {
					uploadProgress.delete(file.name);
					uploadProgress = uploadProgress;
				}, 1000);
			}
		}

		uploading = false;
	}
</script>

<Card.Root
	class="border-2 border-dashed transition-colors {isDragging ? 'border-primary bg-primary/5' : 'border-muted-foreground/25'}"
	ondragenter={handleDragEnter}
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
	<Card.Content class="py-12 text-center">
		<div class="flex flex-col items-center gap-4">
			{#if uploading}
				<LoaderIcon class="h-12 w-12 text-muted-foreground animate-spin" />
				<p class="text-sm text-muted-foreground">Uploading...</p>
				{#each Array.from(uploadProgress.entries()) as [name, progress]}
					<div class="w-full max-w-xs">
						<p class="text-xs text-left mb-1">{name}</p>
						<div class="h-2 bg-muted rounded-full overflow-hidden">
							<div class="h-full bg-primary transition-all" style="width: {progress}%"></div>
						</div>
					</div>
				{/each}
			{:else}
				<UploadIcon class="h-12 w-12 text-muted-foreground" />
				<div>
					<p class="text-sm font-medium">Drag and drop files here</p>
					<p class="text-xs text-muted-foreground mt-1">or</p>
				</div>
				<Button onclick={() => fileInput.click()}>
					Browse Files
				</Button>
			{/if}
		</div>
	</Card.Content>
</Card.Root>

<input
	bind:this={fileInput}
	type="file"
	multiple
	class="hidden"
	onchange={handleFileSelect}
/>

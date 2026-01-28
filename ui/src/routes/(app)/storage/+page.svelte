<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin, files, type Schema, type FileMetadata } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import { Button } from '$ui/button';
	import { Switch } from '$ui/switch';
	import { toast } from 'svelte-sonner';
	import BucketSidebar from '$lib/components/storage/BucketSidebar.svelte';
	import FileBrowser from '$lib/components/storage/FileBrowser.svelte';
	import UploadZone from '$lib/components/storage/UploadZone.svelte';
	import FilePreview from '$lib/components/storage/FilePreview.svelte';
	import FileFilters from '$lib/components/storage/FileFilters.svelte';
	import TableIcon from 'lucide-svelte/icons/table';
	import GridIcon from 'lucide-svelte/icons/grid-2x2';

	const queryClient = useQueryClient();

	let selectedBucket = $state<string | null>(null);
	let viewMode = $state<'table' | 'grid'>('table');
	let search = $state('');
	let mimeType = $state('');
	let selectedIds = $state<string[]>([]);
	let previewFile = $state<FileMetadata | null>(null);
	let previewOpen = $state(false);

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const filesQuery = createQuery(() => ({
		queryKey: ['files', selectedBucket, search, mimeType],
		queryFn: async () => {
			if (!selectedBucket) return null;
			const result = await files.list(selectedBucket, { search, mime_type: mimeType });
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: !!selectedBucket
	}));

	const deleteMutation = createMutation(() => ({
		mutationFn: async (ids: string[]) => {
			if (!selectedBucket) throw new Error('No bucket selected');
			const result = await files.deleteBatch(selectedBucket, ids);
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: (data) => {
			toast.success(`Deleted ${data.deleted} file(s)`);
			if (data.failed.length > 0) {
				toast.error(`Failed to delete ${data.failed.length} file(s)`);
			}
			queryClient.invalidateQueries({ queryKey: ['files', selectedBucket] });
			selectedIds = [];
		},
		onError: (error: Error) => {
			toast.error(`Failed to delete files: ${error.message}`);
		}
	}));

	function handleBucketSelect(name: string) {
		selectedBucket = name;
		search = '';
		mimeType = '';
		selectedIds = [];
	}

	function handleUploadComplete(file: FileMetadata) {
		queryClient.invalidateQueries({ queryKey: ['files', selectedBucket] });
	}

	function handleDelete(ids: string[]) {
		if (confirm(`Delete ${ids.length} file(s)?`)) {
			deleteMutation.mutate(ids);
		}
	}

	function handlePreview(file: FileMetadata) {
		previewFile = file;
		previewOpen = true;
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	<div class="grid grid-cols-[280px_1fr] gap-6">
		<!-- Left Sidebar: Buckets -->
		<div class="border-r pr-6">
			<h2 class="text-lg font-semibold mb-4">Buckets</h2>
			{#if schemaQuery.isPending}
				<Skeleton class="h-20 w-full" />
			{:else if schemaQuery.data?.buckets}
				<BucketSidebar
					buckets={schemaQuery.data.buckets}
					{selectedBucket}
					onSelect={handleBucketSelect}
				/>
			{:else}
				<p class="text-sm text-muted-foreground">No buckets defined</p>
			{/if}
		</div>

		<!-- Right Content: File Browser -->
		<div class="space-y-4">
			{#if !selectedBucket}
				<Card.Root class="flex items-center justify-center py-16">
					<Card.Content class="text-center">
						<p class="text-muted-foreground">Select a bucket to view files</p>
					</Card.Content>
				</Card.Root>
			{:else}
				<div class="flex items-center justify-between">
					<h2 class="text-lg font-semibold">{selectedBucket}</h2>
					<div class="flex items-center gap-2">
						<Button
							variant={viewMode === 'table' ? 'default' : 'outline'}
							size="sm"
							onclick={() => (viewMode = 'table')}
						>
							<TableIcon class="h-4 w-4" />
						</Button>
						<Button
							variant={viewMode === 'grid' ? 'default' : 'outline'}
							size="sm"
							onclick={() => (viewMode = 'grid')}
						>
							<GridIcon class="h-4 w-4" />
						</Button>
					</div>
				</div>

				<UploadZone bucket={selectedBucket} onUploadComplete={handleUploadComplete} />

				<FileFilters
					{search}
					{mimeType}
					onSearchChange={(v) => (search = v)}
					onMimeTypeChange={(v) => (mimeType = v)}
				/>

				{#if filesQuery.isPending}
					<Skeleton class="h-64 w-full" />
				{:else if filesQuery.isError}
					<Card.Root class="border-destructive/50">
						<Card.Content class="py-4">
							<p class="text-sm text-destructive">Failed to load files</p>
						</Card.Content>
					</Card.Root>
				{:else if filesQuery.data}
					<div class="space-y-4">
						{#if selectedIds.length > 0}
							<div class="flex items-center gap-2">
								<p class="text-sm text-muted-foreground">{selectedIds.length} selected</p>
								<Button
									variant="destructive"
									size="sm"
									onclick={() => handleDelete(selectedIds)}
									disabled={deleteMutation.isPending}
								>
									Delete Selected
								</Button>
							</div>
						{/if}

						<FileBrowser
							bucket={selectedBucket}
							files={filesQuery.data.files}
							{viewMode}
							{selectedIds}
							onSelectionChange={(ids) => (selectedIds = ids)}
							onDelete={handleDelete}
							onPreview={handlePreview}
						/>
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>

{#if selectedBucket && previewFile}
	<FilePreview
		file={previewFile}
		open={previewOpen}
		bucket={selectedBucket}
		onClose={() => (previewOpen = false)}
	/>
{/if}

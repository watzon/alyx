<script lang="ts">
	import { page } from '$app/stores';
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { collections, admin, type Field, type ApiError } from '$lib/api/client';
	import * as Card from '$ui/card';
	import * as Table from '$ui/table';
	import * as AlertDialog from '$ui/alert-dialog';
	import { Skeleton } from '$ui/skeleton';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { toast } from 'svelte-sonner';

	function getErrorMessage(err: unknown): string {
		if (err && typeof err === 'object' && 'error' in err) {
			const apiErr = err as ApiError;
			return apiErr.error || apiErr.message || 'Unknown error';
		}
		if (err instanceof Error) {
			return err.message;
		}
		return 'Unknown error';
	}
	import RefreshCwIcon from 'lucide-svelte/icons/refresh-cw';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import Trash2Icon from 'lucide-svelte/icons/trash-2';
	import Loader2Icon from 'lucide-svelte/icons/loader-2';
	import CheckIcon from 'lucide-svelte/icons/check';
	import XIcon from 'lucide-svelte/icons/x';
	import ChevronLeftIcon from 'lucide-svelte/icons/chevron-left';
	import ChevronRightIcon from 'lucide-svelte/icons/chevron-right';
	import ChevronsLeftIcon from 'lucide-svelte/icons/chevrons-left';
	import ChevronsRightIcon from 'lucide-svelte/icons/chevrons-right';
	import SearchIcon from 'lucide-svelte/icons/search';

	const queryClient = useQueryClient();
	const collectionName = $derived($page.params.collection);

	let pageIndex = $state(1);
	let pageSize = $state(50);
	let search = $state('');

	const schemaQuery = createQuery(() => ({
		queryKey: ['schema'],
		queryFn: async () => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const collectionFields = $derived.by(() => {
		if (!schemaQuery.data || !collectionName) return [];
		const collection = schemaQuery.data.collections.find((c) => c.name === collectionName);
		return collection?.fields || [];
	});

	const editableFields = $derived(collectionFields.filter((f) => !f.primary));

	const docsQuery = createQuery(() => ({
		queryKey: ['collections', collectionName, 'documents', pageIndex, pageSize, search],
		queryFn: async () => {
			if (!collectionName) throw new Error('Collection name is required');
			const result = await collections.list(collectionName, {
				page: pageIndex,
				perPage: pageSize,
				search: search || undefined
			});
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: !!collectionName
	}));

	const updateMutation = createMutation(() => ({
		mutationFn: async (args: { id: string; data: Record<string, unknown> }) => {
			if (!collectionName) throw new Error('Collection name is required');
			const result = await collections.update(collectionName, args.id, args.data);
			if (result.error) throw result.error;
			return result.data;
		},
		onSuccess: () => {
			toast.success('Document updated');
			queryClient.invalidateQueries({ queryKey: ['collections', collectionName] });
			editingCell = null;
		},
		onError: (err: unknown) => {
			toast.error('Failed to update', { description: getErrorMessage(err) });
		}
	}));

	const createDocMutation = createMutation(() => ({
		mutationFn: async (data: Record<string, unknown>) => {
			if (!collectionName) throw new Error('Collection name is required');
			const result = await collections.create(collectionName, data);
			if (result.error) throw result.error;
			return result.data;
		},
		onSuccess: () => {
			toast.success('Document created');
			queryClient.invalidateQueries({ queryKey: ['collections', collectionName] });
			draftRow = null;
		},
		onError: (err: unknown) => {
			toast.error('Failed to create', { description: getErrorMessage(err) });
		}
	}));

	const isCreating = $derived(createDocMutation.isPending);

	const deleteMutation = createMutation(() => ({
		mutationFn: async (id: string) => {
			if (!collectionName) throw new Error('Collection name is required');
			const result = await collections.delete(collectionName, id);
			if (result.error) throw result.error;
			return id;
		},
		onSuccess: () => {
			toast.success('Document deleted');
			queryClient.invalidateQueries({ queryKey: ['collections', collectionName] });
			itemToDelete = null;
		},
		onError: (err: unknown) => {
			toast.error('Failed to delete', { description: getErrorMessage(err) });
		}
	}));

	let editingCell = $state<{ id: string; field: string } | null>(null);
	let editValue = $state<unknown>(null);
	let originalValue = $state<unknown>(null);
	let itemToDelete = $state<string | null>(null);
	let draftRow = $state<Record<string, unknown> | null>(null);

	function startEditing(item: Record<string, unknown>, field: Field) {
		if (field.primary) return;
		editingCell = { id: item.id as string, field: field.name };
		editValue = item[field.name];
		originalValue = item[field.name];
	}

	function cancelEditing() {
		editingCell = null;
		editValue = null;
		originalValue = null;
	}

	function saveEdit() {
		if (!editingCell) return;
		if (editValue === originalValue) {
			cancelEditing();
			return;
		}
		updateMutation.mutate({
			id: editingCell.id,
			data: { [editingCell.field]: editValue }
		});
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			saveEdit();
		} else if (e.key === 'Escape') {
			cancelEditing();
		}
	}

	function handleBlur() {
		if (editValue !== originalValue) {
			saveEdit();
		} else {
			cancelEditing();
		}
	}

	function addDraftRow() {
		const draft: Record<string, unknown> = { _draft: true };
		for (const field of editableFields) {
			draft[field.name] = getDefaultValue(field);
		}
		draftRow = draft;
	}

	function cancelDraft() {
		draftRow = null;
	}

	function saveDraft() {
		if (!draftRow) return;
		const data: Record<string, unknown> = {};
		for (const field of editableFields) {
			if (draftRow[field.name] !== undefined && draftRow[field.name] !== '') {
				data[field.name] = draftRow[field.name];
			}
		}
		createDocMutation.mutate(data);
	}

	function updateDraftField(fieldName: string, value: unknown) {
		if (!draftRow) return;
		draftRow = { ...draftRow, [fieldName]: value };
	}

	function getDefaultValue(field: Field): unknown {
		if (field.default !== undefined) return field.default;
		switch (field.type) {
			case 'bool':
				return false;
			case 'int':
			case 'float':
				return null;
			case 'json':
				return '';
			default:
				return '';
		}
	}

	function formatValue(value: unknown, type: string): string {
		if (value === null || value === undefined) return '-';
		if (type === 'bool') return value ? 'Yes' : 'No';
		if (typeof value === 'object') return JSON.stringify(value);
		return String(value);
	}

	function truncate(str: string, maxLength: number = 50): string {
		if (str.length <= maxLength) return str;
		return str.slice(0, maxLength) + '...';
	}

	const totalPages = $derived(Math.max(1, Math.ceil((docsQuery.data?.total || 0) / pageSize)));
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{collectionName}</h1>
			<p class="text-sm text-muted-foreground">
				{#if docsQuery.data}
					{docsQuery.data.total} document{docsQuery.data.total === 1 ? '' : 's'}
				{:else}
					Loading...
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			<Button size="sm" onclick={addDraftRow} disabled={!!draftRow}>
				<PlusIcon class="h-3.5 w-3.5 mr-1.5" />
				Add Row
			</Button>
			<Button variant="outline" size="sm" onclick={() => docsQuery.refetch()}>
				<RefreshCwIcon class="h-3.5 w-3.5 mr-1.5" />
				Refresh
			</Button>
		</div>
	</div>

	<div class="flex items-center gap-4 mb-4">
		<div class="relative flex-1 max-w-sm">
			<SearchIcon class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
			<Input
				type="search"
				placeholder="Search documents..."
				class="pl-9"
				value={search}
				oninput={(e) => {
					search = e.currentTarget.value;
					pageIndex = 1;
				}}
			/>
		</div>
	</div>

	{#if docsQuery.isPending || schemaQuery.isPending}
		<Card.Root class="flex items-center justify-center py-16">
			<Skeleton class="h-32 w-32" />
		</Card.Root>
	{:else if docsQuery.isError || schemaQuery.isError}
		<Card.Root class="border-destructive/50 flex items-center justify-center py-16">
			<p class="text-sm text-destructive">
				Failed to load: {docsQuery.error?.message || schemaQuery.error?.message}
			</p>
		</Card.Root>
	{:else}
		{@const items = docsQuery.data?.docs || []}
		{@const hasData = items.length > 0 || draftRow}
		<Card.Root class="overflow-hidden">
			{#if hasData}
				<div class="overflow-auto">
					<Table.Root>
						<Table.Header class="sticky top-0 z-20 bg-card shadow-[0_1px_0_0_hsl(var(--border))]">
							<Table.Row class="border-b-0">
								<Table.Head class="w-[60px]"></Table.Head>
								{#each collectionFields as field}
									<Table.Head class="whitespace-nowrap min-w-[150px]">
										<div class="flex items-center gap-2">
											<span class="text-xs font-medium">{field.name}</span>
											<span class="text-[10px] text-muted-foreground font-normal px-1 py-0.5 rounded bg-muted">
												{field.type}
											</span>
										</div>
									</Table.Head>
								{/each}
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#if draftRow}
								<Table.Row class="bg-primary/5 border-primary/20">
									<Table.Cell class="sticky left-0 z-10 bg-primary/5 p-1">
										<div class="flex items-center gap-1">
											<Button
												variant="ghost"
												size="icon"
												class="h-7 w-7 text-primary hover:text-primary hover:bg-primary/20"
												onclick={saveDraft}
												disabled={isCreating}
											>
												{#if isCreating}
													<Loader2Icon class="h-3.5 w-3.5 animate-spin" />
												{:else}
													<CheckIcon class="h-3.5 w-3.5" />
												{/if}
											</Button>
											<Button
												variant="ghost"
												size="icon"
												class="h-7 w-7 text-muted-foreground hover:text-destructive"
												onclick={cancelDraft}
											>
												<XIcon class="h-3.5 w-3.5" />
											</Button>
										</div>
									</Table.Cell>
									{#each collectionFields as field}
										<Table.Cell class="p-0 min-w-[150px] h-[40px]">
											{#if field.primary}
												<div class="px-3 py-2 text-muted-foreground text-sm italic">
													(auto)
												</div>
											{:else if field.type === 'bool'}
												<div class="flex items-center justify-center h-full">
													<input
														type="checkbox"
														class="h-4 w-4 rounded border-input bg-background"
														checked={draftRow[field.name] as boolean}
														onchange={(e) => updateDraftField(field.name, e.currentTarget.checked)}
													/>
												</div>
											{:else}
												<Input
													type={field.type === 'int' || field.type === 'float' ? 'number' : 'text'}
													value={draftRow[field.name] as string}
													oninput={(e) => updateDraftField(field.name, e.currentTarget.value)}
													class="h-full w-full rounded-none border-0 bg-transparent px-3 focus-visible:ring-1 focus-visible:ring-primary"
													placeholder={field.nullable ? '(optional)' : '(required)'}
												/>
											{/if}
										</Table.Cell>
									{/each}
								</Table.Row>
							{/if}
							{#each items as item (item.id)}
								<Table.Row class="hover:bg-muted/30">
									<Table.Cell class="sticky left-0 z-10 bg-card p-1">
										<Button
											variant="ghost"
											size="icon"
											class="h-7 w-7 text-muted-foreground hover:text-destructive"
											onclick={() => (itemToDelete = item.id as string)}
										>
											<Trash2Icon class="h-3.5 w-3.5" />
										</Button>
									</Table.Cell>
									{#each collectionFields as field}
										{@const isEditing = editingCell?.id === item.id && editingCell?.field === field.name}
										<Table.Cell
											class="p-0 relative min-w-[150px] h-[40px] {field.primary ? '' : 'cursor-pointer'}"
											onclick={() => !isEditing && startEditing(item, field)}
										>
											{#if isEditing}
												<div class="absolute inset-0 z-20">
													{#if field.type === 'bool'}
														<div class="flex items-center justify-center h-full bg-background">
															<input
																type="checkbox"
																class="h-4 w-4 rounded border-input"
																checked={editValue as boolean}
																onchange={(e) => {
																	editValue = e.currentTarget.checked;
																	saveEdit();
																}}
															/>
														</div>
													{:else if field.type === 'json'}
														<textarea
															value={typeof editValue === 'string' ? editValue : JSON.stringify(editValue)}
															oninput={(e) => (editValue = e.currentTarget.value)}
															class="h-full w-full min-h-[80px] resize-none border-2 border-primary p-2 bg-background text-sm font-mono focus:outline-none"
															onkeydown={handleKeyDown}
															onblur={handleBlur}
														></textarea>
													{:else}
														<Input
															type={field.type === 'int' || field.type === 'float' ? 'number' : 'text'}
															value={editValue as string}
															oninput={(e) => (editValue = e.currentTarget.value)}
															class="h-full w-full rounded-none border-2 border-primary px-3 focus-visible:ring-0"
															onkeydown={handleKeyDown}
															onblur={handleBlur}
														/>
													{/if}
												</div>
											{:else}
												<div class="px-3 py-2 h-full w-full flex items-center {field.primary ? '' : 'hover:bg-muted/50'}">
													<span class="font-mono text-sm truncate">
														{truncate(formatValue(item[field.name], field.type))}
													</span>
												</div>
											{/if}
										</Table.Cell>
									{/each}
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{:else}
				<div class="flex-1 flex flex-col items-center justify-center text-center p-8">
					<div class="rounded-full bg-muted p-4 mb-4">
						<PlusIcon class="h-8 w-8 text-muted-foreground" />
					</div>
					<h3 class="text-sm font-medium mb-1">No documents yet</h3>
					<p class="text-sm text-muted-foreground mb-4">
						Get started by adding your first row
					</p>
					<Button size="sm" onclick={addDraftRow}>
						<PlusIcon class="h-3.5 w-3.5 mr-1.5" />
						Add Row
					</Button>
				</div>
			{/if}

			{#if hasData}
				<div class="flex items-center justify-between px-4 py-3 border-t bg-card">
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<span>Rows per page</span>
						<select
							class="h-8 w-[70px] rounded-md border border-input bg-background px-2 py-1 text-sm"
							value={String(pageSize)}
							onchange={(e) => {
								pageSize = Number(e.currentTarget.value);
								pageIndex = 1;
							}}
						>
							<option value="10">10</option>
							<option value="25">25</option>
							<option value="50">50</option>
							<option value="100">100</option>
						</select>
					</div>

					<div class="flex items-center gap-4">
						<span class="text-sm text-muted-foreground">
							Page {pageIndex} of {totalPages}
						</span>
						<div class="flex items-center gap-1">
							<Button
								variant="outline"
								size="icon"
								class="h-8 w-8"
								onclick={() => (pageIndex = 1)}
								disabled={pageIndex === 1}
							>
								<ChevronsLeftIcon class="h-4 w-4" />
							</Button>
							<Button
								variant="outline"
								size="icon"
								class="h-8 w-8"
								onclick={() => pageIndex--}
								disabled={pageIndex === 1}
							>
								<ChevronLeftIcon class="h-4 w-4" />
							</Button>
							<Button
								variant="outline"
								size="icon"
								class="h-8 w-8"
								onclick={() => pageIndex++}
								disabled={pageIndex >= totalPages}
							>
								<ChevronRightIcon class="h-4 w-4" />
							</Button>
							<Button
								variant="outline"
								size="icon"
								class="h-8 w-8"
								onclick={() => (pageIndex = totalPages)}
								disabled={pageIndex >= totalPages}
							>
								<ChevronsRightIcon class="h-4 w-4" />
							</Button>
						</div>
					</div>
				</div>
			{/if}
		</Card.Root>
	{/if}

	<AlertDialog.Root open={!!itemToDelete} onOpenChange={(open) => !open && (itemToDelete = null)}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Delete document?</AlertDialog.Title>
				<AlertDialog.Description>
					This action cannot be undone. This will permanently delete the document.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
				<AlertDialog.Action
					class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
					onclick={() => itemToDelete && deleteMutation.mutate(itemToDelete)}
				>
					Delete
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
</div>

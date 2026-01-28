<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin, type Schema, type Collection, type PendingChange, type SchemaChange } from '$lib/api/client';
	import { configStore } from '$lib/stores/config.svelte';
	import * as Card from '$ui/card';
	import * as Tabs from '$ui/tabs';
	import * as Tooltip from '$ui/tooltip';
	import * as AlertDialog from '$ui/alert-dialog';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import { Button } from '$ui/button';
	import { YamlEditor } from '$lib/components/yaml-editor';
	import { SchemaEditor, toEditableSchema, toYamlString, validateSchema, type EditableSchema, type SchemaValidationError } from '$lib/components/schema-editor';
	import { toast } from 'svelte-sonner';
	import KeyIcon from 'lucide-svelte/icons/key';
	import LinkIcon from 'lucide-svelte/icons/link';
	import HashIcon from 'lucide-svelte/icons/hash';
	import BookOpenIcon from 'lucide-svelte/icons/book-open';
	import PencilIcon from 'lucide-svelte/icons/pencil';
	import EyeIcon from 'lucide-svelte/icons/eye';
	import SaveIcon from 'lucide-svelte/icons/save';
	import XIcon from 'lucide-svelte/icons/x';
	import AlertTriangleIcon from 'lucide-svelte/icons/triangle-alert';
	import TrashIcon from 'lucide-svelte/icons/trash-2';
	import { browser } from '$app/environment';

	const queryClient = useQueryClient();

	let isEditMode = $state(false);
	let editedContent = $state('');
	let editableSchema = $state<EditableSchema | null>(null);
	let saveError = $state<string | null>(null);
	let validationErrors = $state<SchemaValidationError[]>([]);
	let activeTab = $state<string>('');
	let showPendingDialog = $state(false);
	let showPreviewDialog = $state(false);
	let previewChanges = $state<{ safe: SchemaChange[]; unsafe: SchemaChange[] }>({ safe: [], unsafe: [] });

	let initialHashFromUrl = browser ? window.location.hash.slice(1) : '';
	let isVisualEditMode = $state(initialHashFromUrl.startsWith('visual-edit'));
	let initialTabFromHash = isVisualEditMode ? initialHashFromUrl.replace('visual-edit:', '') : initialHashFromUrl;

	function sortCollections(collections: Collection[] | undefined): Collection[] {
		if (!collections) return [];
		return [...collections].sort((a, b) => a.name.localeCompare(b.name));
	}

	function updateHashFromTab(tab: string) {
		if (browser && tab) {
			const hash = isVisualEditMode ? `visual-edit:${tab}` : tab;
			window.history.replaceState(null, '', `#${hash}`);
		}
	}

	$effect(() => {
		if (activeTab) {
			updateHashFromTab(activeTab);
		}
	});

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const schemaRawQuery = createQuery(() => ({
		queryKey: ['admin', 'schema', 'raw'],
		queryFn: async () => {
			const result = await admin.schemaRaw.get();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: configStore.isDevMode
	}));

	const pendingChangesQuery = createQuery(() => ({
		queryKey: ['admin', 'schema', 'pending'],
		queryFn: async () => {
			const result = await admin.pendingChanges.list();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: configStore.isDevMode,
		refetchInterval: 5000
	}));

	const confirmChangesMutation = createMutation(() => ({
		mutationFn: async () => {
			const result = await admin.pendingChanges.confirm();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: (data) => {
			toast.success(`Applied ${data.applied} schema change${data.applied !== 1 ? 's' : ''}`);
			showPendingDialog = false;
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema'] });
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema', 'pending'] });
		},
		onError: (error: Error) => {
			toast.error('Failed to apply changes: ' + error.message);
		}
	}));

	const cancelChangesMutation = createMutation(() => ({
		mutationFn: async () => {
			const result = await admin.pendingChanges.cancel();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: () => {
			toast.success('Pending changes cancelled');
			showPendingDialog = false;
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema', 'pending'] });
		},
		onError: (error: Error) => {
			toast.error('Failed to cancel changes: ' + error.message);
		}
	}));

	const sortedCollections = $derived(sortCollections(schemaQuery.data?.collections));
	const hasPendingChanges = $derived(pendingChangesQuery.data?.pending ?? false);
	const pendingChanges = $derived(pendingChangesQuery.data?.changes ?? []);

	$effect(() => {
		if (sortedCollections.length > 0 && !activeTab) {
			const hashCollection = initialTabFromHash;
			const collectionExists = sortedCollections.some((c) => c.name === hashCollection);
			activeTab = collectionExists ? hashCollection : sortedCollections[0].name;
		}
	});

	const previewMutation = createMutation(() => ({
		mutationFn: async (content: string) => {
			const result = await admin.schemaDraft.preview(content);
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: (data) => {
			previewChanges = { safe: data.safeChanges || [], unsafe: data.unsafeChanges || [] };
			showPreviewDialog = true;
		},
		onError: (error: Error) => {
			saveError = error.message;
			toast.error('Failed to validate schema: ' + error.message);
		}
	}));

	const applyMutation = createMutation(() => ({
		mutationFn: async () => {
			const result = await admin.schemaDraft.apply();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: (data) => {
			toast.success(`Applied ${data.safeApplied + data.unsafeApplied} schema change${data.safeApplied + data.unsafeApplied !== 1 ? 's' : ''}`);
			showPreviewDialog = false;
			isEditMode = false;
			isVisualEditMode = false;
			saveError = null;
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema'] });
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema', 'raw'] });
		},
		onError: (error: Error) => {
			toast.error('Failed to apply changes: ' + error.message);
		}
	}));

	const cancelDraftMutation = createMutation(() => ({
		mutationFn: async () => {
			const result = await admin.schemaDraft.cancel();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: () => {
			showPreviewDialog = false;
			previewChanges = { safe: [], unsafe: [] };
		}
	}));

	function enterEditMode() {
		if (schemaRawQuery.data) {
			editedContent = schemaRawQuery.data.content;
			saveError = null;
			isEditMode = true;
		}
	}

	function enterVisualEditMode() {
		if (schemaQuery.data) {
			editableSchema = toEditableSchema(schemaQuery.data);
			isVisualEditMode = true;
			isEditMode = false;
			updateHashFromTab(activeTab);
		}
	}

	function cancelEdit() {
		isVisualEditMode = false;
		editableSchema = null;
		isEditMode = false;
		saveError = null;
		if (schemaRawQuery.data) {
			editedContent = schemaRawQuery.data.content;
		}
		updateHashFromTab(activeTab);
	}

	function saveSchema() {
		previewMutation.mutate(editedContent);
	}

	function saveVisualSchema() {
		if (editableSchema) {
			const errors = validateSchema(editableSchema);
			if (errors.length > 0) {
				validationErrors = errors;
				toast.error(`Schema has ${errors.length} validation error${errors.length > 1 ? 's' : ''}`);
				return;
			}
			validationErrors = [];
			saveError = null;
			const yamlContent = toYamlString(editableSchema);
			previewMutation.mutate(yamlContent);
		}
	}

	function getFieldTypeColor(type: string): string {
		switch (type) {
			case 'id':
			case 'uuid':
				return 'bg-purple-500/10 text-purple-500';
			case 'string':
			case 'text':
				return 'bg-green-500/10 text-green-500';
			case 'int':
			case 'float':
				return 'bg-blue-500/10 text-blue-500';
			case 'bool':
				return 'bg-amber-500/10 text-amber-500';
			case 'timestamp':
				return 'bg-cyan-500/10 text-cyan-500';
			case 'json':
				return 'bg-pink-500/10 text-pink-500';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}

	function getChangeTypeColor(type: string): string {
		if (type.includes('drop')) {
			return 'bg-red-500/10 text-red-500 border-red-500/20';
		}
		if (type.includes('modify')) {
			return 'bg-orange-500/10 text-orange-500 border-orange-500/20';
		}
		return 'bg-amber-500/10 text-amber-500 border-amber-500/20';
	}

	function formatChangeType(type: string): string {
		return type.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	{#if configStore.isDevMode && hasPendingChanges}
		<div class="rounded-lg border border-amber-500/50 bg-amber-500/10 p-4">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-3">
					<AlertTriangleIcon class="h-5 w-5 text-amber-500" />
					<div>
						<p class="font-medium text-amber-500">Pending Schema Changes</p>
						<p class="text-sm text-muted-foreground">
							{pendingChanges.length} unsafe change{pendingChanges.length !== 1 ? 's' : ''} require{pendingChanges.length === 1 ? 's' : ''} your confirmation
						</p>
					</div>
				</div>
				<Button variant="outline" size="sm" onclick={() => showPendingDialog = true}>
					Review Changes
				</Button>
			</div>
		</div>
	{/if}

	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Schema</h1>
			<p class="text-sm text-muted-foreground">
				{#if isEditMode}
					Editing schema.yaml
				{:else if isVisualEditMode}
					Visual schema editor
				{:else}
					View your database schema definitions
				{/if}
			</p>
		</div>
		{#if configStore.isDevMode}
			<div class="flex items-center gap-2">
				{#if isEditMode || isVisualEditMode}
					<Button variant="outline" size="sm" onclick={cancelEdit} disabled={previewMutation.isPending}>
						<XIcon class="h-4 w-4 mr-2" />
						Cancel
					</Button>
					<Button size="sm" onclick={isEditMode ? saveSchema : saveVisualSchema} disabled={previewMutation.isPending}>
						<SaveIcon class="h-4 w-4 mr-2" />
						{previewMutation.isPending ? 'Validating...' : 'Save'}
					</Button>
				{:else}
					<Button
						variant="outline"
						size="sm"
						onclick={enterEditMode}
						disabled={schemaRawQuery.isPending}
					>
						<PencilIcon class="h-4 w-4 mr-2" />
						Edit Schema
					</Button>
					<Button
						variant="outline"
						size="sm"
						onclick={enterVisualEditMode}
						disabled={schemaRawQuery.isPending}
					>
						<PencilIcon class="h-4 w-4 mr-2" />
						Visual Edit
					</Button>
				{/if}
			</div>
		{/if}
	</div>

	{#if isEditMode}
		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2">
					<PencilIcon class="h-5 w-5" />
					Edit Schema
				</Card.Title>
				<Card.Description>
					{#if schemaRawQuery.data?.path}
						Editing: <code class="font-mono text-xs bg-muted px-1 py-0.5 rounded">{schemaRawQuery.data.path}</code>
					{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<YamlEditor
					bind:value={editedContent}
					error={saveError}
					onchange={() => { saveError = null; }}
				/>
			</Card.Content>
		</Card.Root>
	{:else if isVisualEditMode && editableSchema}
		<SchemaEditor
			schema={editableSchema}
			onchange={(s) => { editableSchema = s; validationErrors = []; }}
			initialCollection={activeTab}
			disabled={previewMutation.isPending}
			errors={validationErrors}
		/>
	{:else if schemaQuery.isPending}
		<Card.Root>
			<Card.Content class="py-6">
				<Skeleton class="h-48 w-full" />
			</Card.Content>
		</Card.Root>
	{:else if schemaQuery.isError}
		<Card.Root class="border-destructive">
			<Card.Content class="py-6">
				<p class="text-destructive">Failed to load schema</p>
			</Card.Content>
		</Card.Root>
	{:else if schemaQuery.data}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4 mb-6">
			{#each sortedCollections as collection}
				<button
					class="text-left p-4 rounded-lg border transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-border/20 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30 {activeTab === collection.name ? '!bg-muted/30 !backdrop-blur-xl !border-border/40' : ''}"
					onclick={() => activeTab = collection.name}
				>
					<span class="font-medium truncate">{collection.name}</span>
				</button>
			{/each}
		</div>

		<Tabs.Root bind:value={activeTab}>
			{#each sortedCollections as collection}
				{@const docsUrl = configStore.getCollectionDocsUrl(collection.name)}
				<Tabs.Content value={collection.name} class="space-y-4 mt-0">
					<Card.Root>
						<Card.Header class="flex flex-row items-center justify-between space-y-0">
							<Card.Title>Fields</Card.Title>
							{#if docsUrl}
								<Tooltip.Root>
									<Tooltip.Trigger>
										<a
											href={docsUrl}
											target="_blank"
											rel="noopener noreferrer"
											class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
										>
											<BookOpenIcon class="h-4 w-4" />
											<span>API Docs</span>
										</a>
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p>View API documentation for {collection.name}</p>
									</Tooltip.Content>
								</Tooltip.Root>
							{/if}
						</Card.Header>
						<Card.Content>
							<div class="space-y-2">
								{#each collection.fields ?? [] as field}
									<div
										class="flex items-center justify-between rounded-md border border-border p-3"
									>
										<div class="flex items-center gap-3">
											<span class="font-mono text-sm">{field.name}</span>
											<Badge variant="outline" class={getFieldTypeColor(field.type)}>
												{field.type}
											</Badge>
										</div>
										<div class="flex items-center gap-2">
											{#if field.primary}
												<Badge variant="secondary" class="gap-1">
													<KeyIcon class="h-3 w-3" />
													Primary
												</Badge>
											{/if}
											{#if field.references}
												<Badge variant="secondary" class="gap-1">
													<LinkIcon class="h-3 w-3" />
													{field.references}
												</Badge>
											{/if}
											{#if field.unique}
												<Badge variant="outline">Unique</Badge>
											{/if}
											{#if field.nullable}
												<Badge variant="outline">Nullable</Badge>
											{/if}
											{#if field.index}
												<Badge variant="outline" class="gap-1">
													<HashIcon class="h-3 w-3" />
													Indexed
												</Badge>
											{/if}
										</div>
									</div>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>

					{#if collection.indexes?.length}
						<Card.Root>
							<Card.Header>
								<Card.Title>Indexes</Card.Title>
							</Card.Header>
							<Card.Content>
								<div class="space-y-2">
									{#each collection.indexes ?? [] as index}
										<div class="flex items-center justify-between rounded-md border border-border p-3">
											<span class="font-mono text-sm">{index.name}</span>
											<div class="flex items-center gap-2">
												<span class="text-sm text-muted-foreground">
													{index.fields?.join(', ') ?? ''}
												</span>
												{#if index.unique}
													<Badge variant="outline">Unique</Badge>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</Card.Content>
						</Card.Root>
					{/if}

					{#if collection.rules}
						<Card.Root>
							<Card.Header>
								<Card.Title>Access Rules</Card.Title>
							</Card.Header>
							<Card.Content>
								<div class="space-y-2">
									{#each Object.entries(collection.rules) as [operation, rule]}
										<div class="rounded-md border border-border p-3">
											<div class="flex items-center gap-2 mb-2">
												<Badge variant="secondary">{operation}</Badge>
											</div>
											<code class="text-sm font-mono text-muted-foreground">{rule}</code>
										</div>
									{/each}
								</div>
							</Card.Content>
						</Card.Root>
					{/if}
				</Tabs.Content>
			{/each}
		</Tabs.Root>
	{/if}
</div>

<AlertDialog.Root bind:open={showPendingDialog}>
	<AlertDialog.Content class="max-w-2xl">
		<AlertDialog.Header>
			<AlertDialog.Title class="flex items-center gap-2">
				<AlertTriangleIcon class="h-5 w-5 text-amber-500" />
				Confirm Schema Changes
			</AlertDialog.Title>
			<AlertDialog.Description>
				The following changes may result in data loss. Please review carefully before applying.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="max-h-80 overflow-y-auto space-y-2 py-4">
			{#each pendingChanges as change}
				<div class="flex items-start gap-3 rounded-md border p-3">
					<Badge variant="outline" class={getChangeTypeColor(change.type)}>
						{formatChangeType(change.type)}
					</Badge>
					<div class="flex-1 min-w-0">
						<p class="font-mono text-sm">
							{change.collection}{change.field ? `.${change.field}` : ''}
						</p>
						<p class="text-sm text-muted-foreground">{change.description}</p>
					</div>
					{#if change.type.includes('drop')}
						<TrashIcon class="h-4 w-4 text-red-500 shrink-0" />
					{/if}
				</div>
			{/each}
		</div>
		<AlertDialog.Footer>
			<AlertDialog.Cancel 
				disabled={confirmChangesMutation.isPending || cancelChangesMutation.isPending}
				onclick={() => cancelChangesMutation.mutate()}
			>
				Cancel Changes
			</AlertDialog.Cancel>
			<AlertDialog.Action
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
				disabled={confirmChangesMutation.isPending || cancelChangesMutation.isPending}
				onclick={() => confirmChangesMutation.mutate()}
			>
				{confirmChangesMutation.isPending ? 'Applying...' : 'Apply Changes'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showPreviewDialog}>
	<AlertDialog.Content class="max-w-3xl max-h-[90vh] flex flex-col">
		<AlertDialog.Header>
			<AlertDialog.Title>Preview Schema Changes</AlertDialog.Title>
			<AlertDialog.Description>
				Review the changes that will be applied to your database schema.
			</AlertDialog.Description>
		</AlertDialog.Header>
		
		<div class="flex-1 overflow-y-auto space-y-4 py-4">
			{#if previewChanges.safe.length > 0}
				<div class="space-y-2">
					<h3 class="font-semibold text-sm text-green-500 flex items-center gap-2">
						Safe Changes ({previewChanges.safe.length})
					</h3>
					<div class="space-y-2">
						{#each previewChanges.safe as change}
							<div class="flex items-start gap-3 rounded-md border border-green-500/20 bg-green-500/5 p-3">
								<Badge variant="outline" class="bg-green-500/10 text-green-500 border-green-500/20">
									{formatChangeType(change.Type)}
								</Badge>
								<div class="flex-1 min-w-0">
									<p class="font-mono text-sm">
										{change.Collection}{change.Field ? `.${change.Field}` : ''}
									</p>
									<p class="text-sm text-muted-foreground">{change.Description}</p>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			{#if previewChanges.unsafe.length > 0}
				<div class="space-y-2">
					<h3 class="font-semibold text-sm text-amber-500 flex items-center gap-2">
						<AlertTriangleIcon class="h-4 w-4" />
						Unsafe Changes ({previewChanges.unsafe.length})
					</h3>
					<div class="space-y-2">
						{#each previewChanges.unsafe as change}
							<div class="flex items-start gap-3 rounded-md border border-amber-500/20 bg-amber-500/5 p-3">
								<Badge variant="outline" class={getChangeTypeColor(change.Type)}>
									{formatChangeType(change.Type)}
								</Badge>
								<div class="flex-1 min-w-0">
									<p class="font-mono text-sm">
										{change.Collection}{change.Field ? `.${change.Field}` : ''}
									</p>
									<p class="text-sm text-muted-foreground">{change.Description}</p>
								</div>
								{#if change.Type.includes('drop')}
									<TrashIcon class="h-4 w-4 text-red-500 shrink-0" />
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{/if}

			{#if previewChanges.safe.length === 0 && previewChanges.unsafe.length === 0}
				<div class="text-center py-8 text-muted-foreground">
					No changes detected
				</div>
			{/if}
		</div>
		
		<AlertDialog.Footer>
			<AlertDialog.Cancel 
				disabled={applyMutation.isPending}
				onclick={() => cancelDraftMutation.mutate()}
			>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action
				class={previewChanges.unsafe.length > 0 ? "bg-destructive text-destructive-foreground hover:bg-destructive/90" : ""}
				disabled={applyMutation.isPending || (previewChanges.safe.length === 0 && previewChanges.unsafe.length === 0)}
				onclick={async (e) => {
					e.preventDefault();
					await applyMutation.mutateAsync();
				}}
			>
				{applyMutation.isPending ? 'Applying...' : `Apply ${previewChanges.safe.length + previewChanges.unsafe.length} Change${previewChanges.safe.length + previewChanges.unsafe.length !== 1 ? 's' : ''}`}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

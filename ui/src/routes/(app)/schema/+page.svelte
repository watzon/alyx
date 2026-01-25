<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin, type Schema, type Collection } from '$lib/api/client';
	import { configStore } from '$lib/stores/config.svelte';
	import * as Card from '$ui/card';
	import * as Tabs from '$ui/tabs';
	import * as Tooltip from '$ui/tooltip';
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
	import { browser } from '$app/environment';

	const queryClient = useQueryClient();

	let isEditMode = $state(false);
	let isVisualEditMode = $state(false);
	let editedContent = $state('');
	let editableSchema = $state<EditableSchema | null>(null);
	let saveError = $state<string | null>(null);
	let validationErrors = $state<SchemaValidationError[]>([]);
	let activeTab = $state<string>('');
	let initialTabFromHash = browser ? window.location.hash.slice(1) : '';

	function sortCollections(collections: Collection[] | undefined): Collection[] {
		if (!collections) return [];
		return [...collections].sort((a, b) => a.name.localeCompare(b.name));
	}

	function updateHashFromTab(tab: string) {
		if (browser && tab) {
			window.history.replaceState(null, '', `#${tab}`);
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

	const sortedCollections = $derived(sortCollections(schemaQuery.data?.collections));

	$effect(() => {
		if (sortedCollections.length > 0 && !activeTab) {
			const hashCollection = initialTabFromHash;
			const collectionExists = sortedCollections.some((c) => c.name === hashCollection);
			activeTab = collectionExists ? hashCollection : sortedCollections[0].name;
		}
	});

	const saveMutation = createMutation(() => ({
		mutationFn: async (content: string) => {
			const result = await admin.schemaRaw.update(content);
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: () => {
			toast.success('Schema saved successfully. Changes will be applied on server restart or hot-reload.');
			isEditMode = false;
			saveError = null;
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema'] });
			queryClient.invalidateQueries({ queryKey: ['admin', 'schema', 'raw'] });
		},
		onError: (error: Error) => {
			saveError = error.message;
			toast.error('Failed to save schema: ' + error.message);
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
	}

	function saveSchema() {
		saveMutation.mutate(editedContent);
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
			saveMutation.mutate(yamlContent);
		}
	}

	function getFieldTypeColor(type: string): string {
		switch (type) {
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
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
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
					<Button variant="outline" size="sm" onclick={cancelEdit} disabled={saveMutation.isPending}>
						<XIcon class="h-4 w-4 mr-2" />
						Cancel
					</Button>
					<Button size="sm" onclick={isEditMode ? saveSchema : saveVisualSchema} disabled={saveMutation.isPending}>
						<SaveIcon class="h-4 w-4 mr-2" />
						{saveMutation.isPending ? 'Saving...' : 'Save'}
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
			disabled={saveMutation.isPending}
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
		<Tabs.Root bind:value={activeTab}>
			<Tabs.List class="w-full justify-start overflow-x-auto">
				{#each sortedCollections as collection}
					<Tabs.Trigger value={collection.name}>{collection.name}</Tabs.Trigger>
				{/each}
			</Tabs.List>

			{#each sortedCollections as collection}
				{@const docsUrl = configStore.getCollectionDocsUrl(collection.name)}
				<Tabs.Content value={collection.name} class="space-y-4">
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

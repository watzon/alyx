<script lang="ts">
	import { Button } from '$ui/button';
	import * as Tabs from '$ui/tabs';
	import CollectionEditor from './collection-editor.svelte';
	import { YamlEditor } from '$lib/components/yaml-editor';
	import {
		type EditableSchema,
		type EditableCollection,
		type SchemaValidationError,
		createEmptyCollection,
		toYamlString
	} from './types';
	import { createHistory } from './use-history.svelte';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import CodeIcon from 'lucide-svelte/icons/code';
	import LayoutGridIcon from 'lucide-svelte/icons/layout-grid';

	interface Props {
		schema: EditableSchema;
		onchange: (schema: EditableSchema) => void;
		initialCollection?: string;
		disabled?: boolean;
		errors?: SchemaValidationError[];
	}

	let { schema, onchange, initialCollection, disabled = false, errors = [] }: Props = $props();

	let viewMode: 'visual' | 'yaml' = $state('visual');
	let activeCollection = $state<string>('');
	let containerEl: HTMLDivElement | undefined = $state();

	// Track schema identity to reset history when parent provides new schema
	let lastSchemaId = $state<string | null>(null);
	// Pass getter function to avoid capturing initial schema value
	const history = createHistory(() => schema);
	
	// Reset history when schema prop changes from parent (not via our own edits)
	$effect(() => {
		const currentId = JSON.stringify(schema.collections.map(c => c._id));
		if (lastSchemaId !== null && lastSchemaId !== currentId) {
			history.reset(schema);
		}
		lastSchemaId = currentId;
	});

	function applyChange(newSchema: EditableSchema) {
		history.push(newSchema);
		onchange(newSchema);
	}

	function handleKeydown(e: KeyboardEvent) {
		const isMac = navigator.platform.toUpperCase().includes('MAC');
		const modKey = isMac ? e.metaKey : e.ctrlKey;

		if (modKey && e.key.toLowerCase() === 'z') {
			e.preventDefault();
			if (e.shiftKey) {
				const redone = history.redo();
				if (redone) onchange(redone);
			} else {
				const undone = history.undo();
				if (undone) onchange(undone);
			}
		} else if (modKey && e.key.toLowerCase() === 'y') {
			e.preventDefault();
			const redone = history.redo();
			if (redone) onchange(redone);
		}
	}

	const sortedCollections = $derived(
		[...schema.collections].sort((a, b) => a.name.localeCompare(b.name))
	);

	function getCollectionErrorCount(collectionId: string): number {
		return errors.filter((e) => e.collectionId === collectionId).length;
	}

	$effect(() => {
		if (sortedCollections.length > 0 && !activeCollection) {
			if (initialCollection) {
				const match = sortedCollections.find((c) => c.name === initialCollection);
				activeCollection = match?._id ?? sortedCollections[0]._id;
			} else {
				activeCollection = sortedCollections[0]._id;
			}
		}
	});

	function updateCollection(id: string, collection: EditableCollection) {
		applyChange({
			...schema,
			collections: schema.collections.map((c) => (c._id === id ? collection : c))
		});
	}

	function deleteCollection(id: string) {
		const newCollections = schema.collections.filter((c) => c._id !== id);
		applyChange({ ...schema, collections: newCollections });

		if (activeCollection === id) {
			if (newCollections.length > 0) {
				activeCollection = newCollections[0]._id;
			} else {
				activeCollection = '';
			}
		} else if (newCollections.length === 0) {
			activeCollection = '';
		}
	}

	function addCollection() {
		const newCollection = createEmptyCollection();
		applyChange({
			...schema,
			collections: [...schema.collections, newCollection]
		});
		activeCollection = newCollection._id;
	}

	const yamlPreview = $derived(toYamlString(schema));

	function handleYamlChange(value: string) {
		// In YAML mode, we don't parse back - this is read-only preview
		// User must switch to visual mode to make changes
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-2">
			<Button
				variant={viewMode === 'visual' ? 'default' : 'outline'}
				size="sm"
				onclick={() => (viewMode = 'visual')}
			>
				<LayoutGridIcon class="h-4 w-4 mr-2" />
				Visual
			</Button>
			<Button
				variant={viewMode === 'yaml' ? 'default' : 'outline'}
				size="sm"
				onclick={() => (viewMode = 'yaml')}
			>
				<CodeIcon class="h-4 w-4 mr-2" />
				YAML Preview
			</Button>
		</div>

		{#if viewMode === 'visual'}
			<Button variant="outline" size="sm" onclick={addCollection} {disabled}>
				<PlusIcon class="h-4 w-4 mr-2" />
				Add Collection
			</Button>
		{/if}
	</div>

	{#if viewMode === 'visual'}
		{#if schema.collections.length === 0}
			<div class="rounded-lg border border-dashed border-border p-12 text-center">
				<p class="text-muted-foreground mb-4">No collections defined yet.</p>
				<Button onclick={addCollection} {disabled}>
					<PlusIcon class="h-4 w-4 mr-2" />
					Create Your First Collection
				</Button>
			</div>
		{:else}
			<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4 mb-6">
				{#each sortedCollections as collection (collection._id)}
					{@const errorCount = getCollectionErrorCount(collection._id)}
					<button
						class="text-left p-4 rounded-lg border transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-border/20 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30 {activeCollection === collection._id ? '!bg-muted/30 !backdrop-blur-xl !border-border/40' : ''}"
						onclick={() => activeCollection = collection._id}
					>
						<div class="flex items-center justify-between gap-2">
							<span class="font-medium truncate">{collection.name || 'Unnamed'}</span>
							{#if errorCount > 0}
								<span class="inline-flex items-center justify-center h-5 min-w-5 px-1 text-xs font-medium rounded-full bg-destructive text-destructive-foreground">
									{errorCount}
								</span>
							{/if}
						</div>
					</button>
				{/each}
			</div>

			<Tabs.Root bind:value={activeCollection}>
				{#each sortedCollections as collection (collection._id)}
					<Tabs.Content value={collection._id} class="mt-0">
						<CollectionEditor
							{collection}
							allCollections={schema.collections}
							buckets={schema.buckets}
							onupdate={(c) => updateCollection(collection._id, c)}
							ondelete={() => deleteCollection(collection._id)}
							{disabled}
							errors={errors.filter((e) => e.collectionId === collection._id)}
						/>
					</Tabs.Content>
				{/each}
			</Tabs.Root>
		{/if}
	{:else}
		<div class="rounded-lg border border-border">
			<div class="px-4 py-2 border-b border-border bg-muted/50">
				<p class="text-sm text-muted-foreground">
					This is a preview of the YAML that will be saved. Switch to Visual mode to make changes.
				</p>
			</div>
			<YamlEditor value={yamlPreview} readonly onchange={handleYamlChange} />
		</div>
	{/if}
</div>

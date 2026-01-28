<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin } from '$lib/api/client';
	import { configStore } from '$lib/stores/config.svelte';
	import * as Card from '$ui/card';
	import * as Table from '$ui/table';
	import * as Tabs from '$ui/tabs';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import * as Dialog from '$ui/dialog';
	import { YamlEditor } from '$lib/components/yaml-editor';
	import { toast } from 'svelte-sonner';
	import KeyIcon from 'lucide-svelte/icons/key';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import TrashIcon from 'lucide-svelte/icons/trash';
	import CopyIcon from 'lucide-svelte/icons/copy';
	import SaveIcon from 'lucide-svelte/icons/save';
	import SettingsIcon from 'lucide-svelte/icons/settings';

	const queryClient = useQueryClient();

	let activeTab = $state('tokens');

	const tokensQuery = createQuery(() => ({
		queryKey: ['admin', 'tokens'],
		queryFn: async () => {
			const result = await admin.tokens.list();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const configRawQuery = createQuery(() => ({
		queryKey: ['admin', 'config', 'raw'],
		queryFn: async () => {
			const result = await admin.configRaw.get();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		enabled: configStore.isDevMode
	}));

	let isCreateDialogOpen = $state(false);
	let newTokenName = $state('');
	let createdToken = $state<string | null>(null);

	let configContent = $state('');
	let configError = $state<string | null>(null);
	let hasConfigChanges = $state(false);

	$effect(() => {
		if (configRawQuery.data && !hasConfigChanges) {
			configContent = configRawQuery.data.content;
		}
	});

	const saveConfigMutation = createMutation(() => ({
		mutationFn: async (content: string) => {
			const result = await admin.configRaw.update(content);
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		},
		onSuccess: () => {
			toast.success('Configuration saved successfully. Restart the server to apply changes.');
			configError = null;
			hasConfigChanges = false;
			queryClient.invalidateQueries({ queryKey: ['admin', 'config', 'raw'] });
		},
		onError: (error: Error) => {
			configError = error.message;
			toast.error('Failed to save configuration: ' + error.message);
		}
	}));

	function handleConfigChange(value: string) {
		configContent = value;
		hasConfigChanges = configContent !== configRawQuery.data?.content;
		configError = null;
	}

	function saveConfig() {
		saveConfigMutation.mutate(configContent);
	}

	function resetConfig() {
		if (configRawQuery.data) {
			configContent = configRawQuery.data.content;
			hasConfigChanges = false;
			configError = null;
		}
	}

	async function handleCreateToken() {
		if (!newTokenName.trim()) return;

		try {
			const result = await admin.tokens.create(newTokenName);
			if (result.error) throw new Error(result.error.message);
			createdToken = result.data!.token;
			newTokenName = '';
			queryClient.invalidateQueries({ queryKey: ['admin', 'tokens'] });
			toast.success('Token created');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create token');
		}
	}

	async function handleDeleteToken(name: string) {
		try {
			const result = await admin.tokens.delete(name);
			if (result.error) throw new Error(result.error.message);
			queryClient.invalidateQueries({ queryKey: ['admin', 'tokens'] });
			toast.success('Token deleted');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete token');
		}
	}

	async function copyToClipboard(text: string) {
		await navigator.clipboard.writeText(text);
		toast.success('Copied to clipboard');
	}

	function closeCreateDialog() {
		isCreateDialogOpen = false;
		createdToken = null;
		newTokenName = '';
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-muted-foreground">Manage API tokens and configuration</p>
	</div>

	<Tabs.Root bind:value={activeTab}>
		<Tabs.List class="!bg-transparent !border-0 !p-0 gap-1">
			<Tabs.Trigger 
				value="tokens"
				class="rounded-t-lg bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-x border-t border-b-0 border-border/20 data-[state=active]:bg-muted/30 data-[state=active]:backdrop-blur-xl data-[state=active]:border-border/40 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30"
			>
				<KeyIcon class="h-4 w-4 mr-2" />
				API Tokens
			</Tabs.Trigger>
			{#if configStore.isDevMode}
				<Tabs.Trigger 
					value="config"
					class="rounded-t-lg bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border-x border-t border-b-0 border-border/20 data-[state=active]:bg-muted/30 data-[state=active]:backdrop-blur-xl data-[state=active]:border-border/40 hover:bg-muted/20 hover:backdrop-blur-xl hover:border-border/30"
				>
					<SettingsIcon class="h-4 w-4 mr-2" />
					Configuration
					{#if hasConfigChanges}
						<Badge variant="secondary" class="ml-2 text-xs">Unsaved</Badge>
					{/if}
				</Tabs.Trigger>
			{/if}
		</Tabs.List>

		<Tabs.Content value="tokens">
			<Card.Root>
				<Card.Header class="flex flex-row items-center justify-between">
					<div>
						<Card.Title>API Tokens</Card.Title>
						<Card.Description>Manage deploy tokens for CI/CD</Card.Description>
					</div>
					<Dialog.Root bind:open={isCreateDialogOpen} onOpenChange={closeCreateDialog}>
						<Dialog.Trigger>
							<Button size="sm">
								<PlusIcon class="mr-2 h-4 w-4" />
								Create Token
							</Button>
						</Dialog.Trigger>
						<Dialog.Content>
							<Dialog.Header>
								<Dialog.Title>Create API Token</Dialog.Title>
								<Dialog.Description>
									{#if createdToken}
										Copy this token now. You won't be able to see it again.
									{:else}
										Enter a name for the new token.
									{/if}
								</Dialog.Description>
							</Dialog.Header>
							{#if createdToken}
								<div class="space-y-4 py-4">
									<div class="flex items-center gap-2 rounded-md border border-border bg-muted p-3">
										<code class="flex-1 overflow-hidden text-ellipsis font-mono text-sm">
											{createdToken}
										</code>
										<Button
											variant="ghost"
											size="sm"
											onclick={() => copyToClipboard(createdToken!)}
										>
											<CopyIcon class="h-4 w-4" />
										</Button>
									</div>
								</div>
								<Dialog.Footer>
									<Button onclick={closeCreateDialog}>Done</Button>
								</Dialog.Footer>
							{:else}
								<div class="space-y-4 py-4">
									<div class="space-y-2">
										<Label for="token-name">Token Name</Label>
										<Input
											id="token-name"
											placeholder="e.g., github-actions"
											bind:value={newTokenName}
										/>
									</div>
								</div>
								<Dialog.Footer>
									<Button variant="outline" onclick={closeCreateDialog}>Cancel</Button>
									<Button onclick={handleCreateToken} disabled={!newTokenName.trim()}>Create</Button>
								</Dialog.Footer>
							{/if}
						</Dialog.Content>
					</Dialog.Root>
				</Card.Header>
				<Card.Content>
					{#if tokensQuery.isPending}
						<div class="space-y-2">
							{#each Array(2) as _}
								<Skeleton class="h-12 w-full" />
							{/each}
						</div>
					{:else if tokensQuery.isError}
						<p class="text-destructive">Failed to load tokens</p>
					{:else if !tokensQuery.data?.tokens?.length}
						<div class="py-8 text-center">
							<KeyIcon class="mx-auto h-8 w-8 text-muted-foreground" />
							<p class="mt-2 text-sm text-muted-foreground">No API tokens created yet</p>
						</div>
					{:else}
						<Table.Root>
							<Table.Header>
								<Table.Row>
									<Table.Head>Name</Table.Head>
									<Table.Head>Created</Table.Head>
									<Table.Head class="w-16"></Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each tokensQuery.data.tokens as token}
									<Table.Row>
										<Table.Cell class="font-mono">{token.name}</Table.Cell>
										<Table.Cell class="text-muted-foreground">
											{new Date(token.created_at).toLocaleDateString()}
										</Table.Cell>
										<Table.Cell>
											<Button
												variant="ghost"
												size="sm"
												onclick={() => handleDeleteToken(token.name)}
											>
												<TrashIcon class="h-4 w-4 text-destructive" />
											</Button>
										</Table.Cell>
									</Table.Row>
								{/each}
							</Table.Body>
						</Table.Root>
					{/if}
				</Card.Content>
			</Card.Root>
		</Tabs.Content>

		{#if configStore.isDevMode}
			<Tabs.Content value="config">
				<Card.Root>
					<Card.Header class="flex flex-row items-center justify-between">
						<div>
							<Card.Title>Server Configuration</Card.Title>
							<Card.Description>
								{#if configRawQuery.data?.path}
									Editing: <code class="font-mono text-xs bg-muted px-1 py-0.5 rounded">{configRawQuery.data.path}</code>
								{:else}
									Edit alyx.yaml configuration
								{/if}
							</Card.Description>
						</div>
						<div class="flex items-center gap-2">
							{#if hasConfigChanges}
								<Button variant="outline" size="sm" onclick={resetConfig}>
									Reset
								</Button>
							{/if}
							<Button
								size="sm"
								onclick={saveConfig}
								disabled={!hasConfigChanges || saveConfigMutation.isPending}
							>
								<SaveIcon class="h-4 w-4 mr-2" />
								{saveConfigMutation.isPending ? 'Saving...' : 'Save'}
							</Button>
						</div>
					</Card.Header>
					<Card.Content>
						{#if configRawQuery.isPending}
							<Skeleton class="h-[400px] w-full" />
						{:else if configRawQuery.isError}
							<div class="py-8 text-center">
								<p class="text-destructive">Failed to load configuration</p>
							</div>
						{:else}
							<YamlEditor
								value={configContent}
								error={configError}
								onchange={handleConfigChange}
							/>
						{/if}
					</Card.Content>
				</Card.Root>
			</Tabs.Content>
		{/if}
	</Tabs.Root>
</div>

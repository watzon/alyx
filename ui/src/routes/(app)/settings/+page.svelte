<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin } from '$lib/api/client';
	import * as Card from '$ui/card';
	import * as Table from '$ui/table';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import * as Dialog from '$ui/dialog';
	import KeyIcon from 'lucide-svelte/icons/key';
	import PlusIcon from 'lucide-svelte/icons/plus';
	import TrashIcon from 'lucide-svelte/icons/trash';
	import CopyIcon from 'lucide-svelte/icons/copy';
	import { toast } from 'svelte-sonner';

	const queryClient = useQueryClient();

	const tokensQuery = createQuery(() => ({
		queryKey: ['admin', 'tokens'],
		queryFn: async () => {
			const result = await admin.tokens.list();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	let isCreateDialogOpen = $state(false);
	let newTokenName = $state('');
	let createdToken = $state<string | null>(null);

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

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-muted-foreground">Manage API tokens and configuration</p>
	</div>

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
</div>

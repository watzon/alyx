<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type Stats, type Schema } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import DatabaseIcon from 'lucide-svelte/icons/database';
	import UsersIcon from 'lucide-svelte/icons/users';
	import CodeIcon from 'lucide-svelte/icons/code';
	import FileTextIcon from 'lucide-svelte/icons/file-text';

	const statsQuery = createQuery(() => ({
		queryKey: ['admin', 'stats'],
		queryFn: async (): Promise<Stats> => {
			const result = await admin.stats();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const schemaQuery = createQuery(() => ({
		queryKey: ['admin', 'schema'],
		queryFn: async (): Promise<Schema> => {
			const result = await admin.schema();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	function formatUptime(seconds: number): string {
		const days = Math.floor(seconds / 86400);
		const hours = Math.floor((seconds % 86400) / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);

		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		return `${minutes}m`;
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>
		<p class="text-sm text-muted-foreground">Overview of your Alyx instance</p>
	</div>

	<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
				<Card.Title class="text-sm font-medium">Collections</Card.Title>
				<DatabaseIcon class="h-4 w-4 text-muted-foreground" />
			</Card.Header>
			<Card.Content>
				{#if schemaQuery.isPending}
					<Skeleton class="h-8 w-16" />
				{:else if schemaQuery.isError}
					<p class="text-destructive">Error</p>
				{:else}
					<p class="text-2xl font-bold">{schemaQuery.data?.collections?.length ?? 0}</p>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
				<Card.Title class="text-sm font-medium">Documents</Card.Title>
				<FileTextIcon class="h-4 w-4 text-muted-foreground" />
			</Card.Header>
			<Card.Content>
				{#if statsQuery.isPending}
					<Skeleton class="h-8 w-16" />
				{:else if statsQuery.isError}
					<p class="text-destructive">Error</p>
				{:else}
					<p class="text-2xl font-bold">{statsQuery.data?.documents ?? 0}</p>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
				<Card.Title class="text-sm font-medium">Users</Card.Title>
				<UsersIcon class="h-4 w-4 text-muted-foreground" />
			</Card.Header>
			<Card.Content>
				{#if statsQuery.isPending}
					<Skeleton class="h-8 w-16" />
				{:else if statsQuery.isError}
					<p class="text-destructive">Error</p>
				{:else}
					<p class="text-2xl font-bold">{statsQuery.data?.users ?? 0}</p>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
				<Card.Title class="text-sm font-medium">Functions</Card.Title>
				<CodeIcon class="h-4 w-4 text-muted-foreground" />
			</Card.Header>
			<Card.Content>
				{#if statsQuery.isPending}
					<Skeleton class="h-8 w-16" />
				{:else if statsQuery.isError}
					<p class="text-destructive">Error</p>
				{:else}
					<p class="text-2xl font-bold">{statsQuery.data?.functions ?? 0}</p>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>

	<div class="grid gap-3 md:grid-cols-2">
		<Card.Root>
			<Card.Header class="pb-3">
				<Card.Title class="text-sm font-medium">System Status</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-3">
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">Status</span>
					<span class="flex items-center gap-1.5">
						<span class="h-1.5 w-1.5 rounded-full bg-success"></span>
						<span class="text-xs font-medium">Healthy</span>
					</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">Uptime</span>
					{#if statsQuery.isPending}
						<Skeleton class="h-3 w-12" />
					{:else if statsQuery.data?.uptime}
						<span class="text-xs font-medium">{formatUptime(statsQuery.data.uptime)}</span>
					{:else}
						<span class="text-xs text-muted-foreground">-</span>
					{/if}
				</div>
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">Version</span>
					<span class="text-xs font-medium font-mono">dev</span>
				</div>
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header class="pb-3">
				<Card.Title class="text-sm font-medium">Quick Actions</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-1.5">
				<a
					href="/_admin/collections"
					class="flex items-center gap-2.5 rounded-md border border-border px-3 py-2 text-xs hover:bg-accent hover:border-foreground/20 transition-colors"
				>
					<DatabaseIcon class="h-3.5 w-3.5 text-primary" />
					<span>Browse Collections</span>
				</a>
				<a
					href="/_admin/schema"
					class="flex items-center gap-2.5 rounded-md border border-border px-3 py-2 text-xs hover:bg-accent hover:border-foreground/20 transition-colors"
				>
					<FileTextIcon class="h-3.5 w-3.5 text-primary" />
					<span>View Schema</span>
				</a>
				<a
					href="/_admin/users"
					class="flex items-center gap-2.5 rounded-md border border-border px-3 py-2 text-xs hover:bg-accent hover:border-foreground/20 transition-colors"
				>
					<UsersIcon class="h-3.5 w-3.5 text-primary" />
					<span>Manage Users</span>
				</a>
			</Card.Content>
		</Card.Root>
	</div>
</div>

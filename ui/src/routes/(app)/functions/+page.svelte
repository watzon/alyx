<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type FunctionInfo } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import { Badge } from '$ui/badge';
	import { Button } from '$ui/button';
	import CodeIcon from 'lucide-svelte/icons/code';
	import PlayIcon from 'lucide-svelte/icons/play';
	import RefreshCwIcon from 'lucide-svelte/icons/refresh-cw';
	import { toast } from 'svelte-sonner';

	const functionsQuery = createQuery(() => ({
		queryKey: ['admin', 'functions'],
		queryFn: async (): Promise<{ functions: FunctionInfo[] }> => {
			const result = await admin.functions.list();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	let isReloading = $state(false);

	async function handleReload() {
		isReloading = true;
		try {
			const result = await admin.functions.reload();
			if (result.error) throw new Error(result.error.message);
			toast.success('Functions reloaded');
			functionsQuery.refetch();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to reload functions');
		} finally {
			isReloading = false;
		}
	}

	function getRuntimeColor(runtime: string): string {
		switch (runtime) {
			case 'node':
			case 'nodejs':
				return 'bg-green-500/10 text-green-500';
			case 'python':
				return 'bg-blue-500/10 text-blue-500';
			case 'go':
				return 'bg-cyan-500/10 text-cyan-500';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Functions</h1>
			<p class="text-sm text-muted-foreground">Manage your serverless functions</p>
		</div>
		<Button variant="outline" size="sm" onclick={handleReload} disabled={isReloading}>
			<RefreshCwIcon class="mr-1.5 h-3.5 w-3.5 {isReloading ? 'animate-spin' : ''}" />
			Reload
		</Button>
	</div>

	{#if functionsQuery.isPending}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each Array(3) as _}
				<Card.Root>
					<Card.Header>
						<Skeleton class="h-4 w-28" />
					</Card.Header>
					<Card.Content>
						<Skeleton class="h-4 w-20" />
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{:else if functionsQuery.isError}
		<Card.Root class="border-destructive/50">
			<Card.Content class="py-4">
				<p class="text-sm text-destructive">Failed to load functions</p>
			</Card.Content>
		</Card.Root>
	{:else if !functionsQuery.data?.functions?.length}
		<Card.Root>
			<Card.Content class="py-10 text-center">
				<CodeIcon class="mx-auto h-10 w-10 text-muted-foreground/50" />
				<h3 class="mt-3 text-sm font-medium">No functions</h3>
				<p class="mt-1 text-sm text-muted-foreground">
					Add functions to the <code class="font-mono text-xs">functions/</code> directory
				</p>
			</Card.Content>
		</Card.Root>
	{:else}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each functionsQuery.data.functions as func}
				<Card.Root>
					<Card.Header class="flex flex-row items-center justify-between space-y-0 pb-2">
						<Card.Title class="text-sm font-medium font-mono">{func.name}</Card.Title>
						<div class="flex items-center gap-2">
							{#if func.enabled}
								<span class="h-1.5 w-1.5 rounded-full bg-success"></span>
							{:else}
								<span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/30"></span>
							{/if}
						</div>
					</Card.Header>
					<Card.Content>
						<div class="flex items-center justify-between">
							<Badge variant="secondary" class={getRuntimeColor(func.runtime)}>
								{func.runtime}
							</Badge>
							<Button variant="ghost" size="sm" class="h-7 text-xs">
								<PlayIcon class="mr-1 h-3 w-3" />
								Test
							</Button>
						</div>
					</Card.Content>
				</Card.Root>
			{/each}
		</div>
	{/if}
</div>

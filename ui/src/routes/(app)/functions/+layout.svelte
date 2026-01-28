<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type FunctionInfo } from '$lib/api/client';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Skeleton } from '$ui/skeleton';
	import RefreshCwIcon from 'lucide-svelte/icons/refresh-cw';
	import SearchIcon from 'lucide-svelte/icons/search';
	import CodeIcon from 'lucide-svelte/icons/code';

	let { children } = $props();

	let searchQuery = $state('');

	const functionsQuery = createQuery(() => ({
		queryKey: ['admin', 'functions'],
		queryFn: async (): Promise<{ functions: FunctionInfo[] }> => {
			const result = await admin.functions.list();
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const filteredFunctions = $derived(
		functionsQuery.data?.functions.filter((f) =>
			f.name.toLowerCase().includes(searchQuery.toLowerCase())
		) ?? []
	);

	const currentFunctionName = $derived((page.params as { name?: string }).name);

	function handleRefresh() {
		functionsQuery.refetch();
	}

	function handleFunctionClick(name: string) {
		goto(`${base}/functions/${name}`);
	}

	function getRuntimeLabel(runtime: string): string {
		switch (runtime) {
			case 'node':
			case 'nodejs':
				return 'Node.js';
			case 'python':
				return 'Python';
			case 'go':
				return 'Go';
			case 'deno':
				return 'Deno';
			case 'bun':
				return 'Bun';
			default:
				return runtime;
		}
	}
</script>

<div class="flex h-[calc(100vh-6.5rem)] -mt-6 -mx-4 sm:-mx-6 lg:-mx-8">
	<aside class="w-72 flex-shrink-0 flex flex-col">
		<div class="p-6">
			<div class="relative">
				<SearchIcon class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
				<Input
					type="text"
					placeholder="Search..."
					class="h-10 pl-9 text-sm bg-muted/40 border-border/50 focus:bg-background"
					bind:value={searchQuery}
				/>
			</div>
		</div>

		<nav class="flex-1 overflow-auto px-4 pb-6">
			{#if functionsQuery.isPending}
				<div class="space-y-1 px-2">
					{#each Array(5) as _}
						<Skeleton class="h-10 w-full rounded-lg" />
					{/each}
				</div>
			{:else if functionsQuery.isError}
				<div class="px-2 py-8 text-center">
					<p class="text-sm text-muted-foreground">Failed to load</p>
					<Button variant="ghost" size="sm" class="mt-2" onclick={handleRefresh}>
						<RefreshCwIcon class="h-3.5 w-3.5 mr-1.5" />
						Retry
					</Button>
				</div>
			{:else if filteredFunctions.length === 0}
				<div class="flex flex-col items-center justify-center py-16 text-center">
					<CodeIcon class="h-10 w-10 text-muted-foreground/20" />
					<p class="mt-4 text-sm text-muted-foreground">
						{searchQuery ? 'No matches found' : 'No functions'}
					</p>
				</div>
			{:else}
				<div class="space-y-0.5">
					{#each filteredFunctions as func}
						<button
							class="group w-full flex items-center justify-between rounded-lg px-3 py-2.5 text-left transition-colors {currentFunctionName === func.name
								? 'bg-muted text-foreground'
								: 'text-muted-foreground hover:bg-muted/50 hover:text-foreground'}"
							onclick={() => handleFunctionClick(func.name)}
						>
							<span class="truncate text-sm {currentFunctionName === func.name ? 'font-medium' : ''}">{func.name}</span>
							<span class="text-xs text-muted-foreground/70 ml-2 shrink-0">{getRuntimeLabel(func.runtime)}</span>
						</button>
					{/each}
				</div>
			{/if}
		</nav>
	</aside>

	<main class="flex-1 overflow-auto border-l border-border/50">
		{@render children()}
	</main>
</div>

<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type FunctionInfo } from '$lib/api/client';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Badge } from '$ui/badge';
	import { Skeleton } from '$ui/skeleton';
	import RefreshCwIcon from 'lucide-svelte/icons/refresh-cw';
	import SearchIcon from 'lucide-svelte/icons/search';
	import CodeIcon from 'lucide-svelte/icons/code';
	import { toast } from 'svelte-sonner';

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

	async function handleRefresh() {
		functionsQuery.refetch();
		toast.success('Functions refreshed');
	}

	function handleFunctionClick(name: string) {
		goto(`${base}/functions/${name}`);
	}

	function getRuntimeColor(runtime: string): string {
		switch (runtime) {
			case 'node':
			case 'nodejs':
				return 'bg-green-500/10 text-green-500 border-green-500/20';
			case 'python':
				return 'bg-blue-500/10 text-blue-500 border-blue-500/20';
			case 'go':
				return 'bg-cyan-500/10 text-cyan-500 border-cyan-500/20';
			case 'deno':
				return 'bg-purple-500/10 text-purple-500 border-purple-500/20';
			case 'bun':
				return 'bg-orange-500/10 text-orange-500 border-orange-500/20';
			default:
				return 'bg-muted text-muted-foreground border-transparent';
		}
	}
</script>

<div class="flex h-[calc(100vh-8rem)]">
	<aside class="w-60 flex flex-col border-r bg-background">
		<div class="flex items-center justify-between border-b px-3 py-2">
			<h2 class="text-sm font-semibold">Functions</h2>
			<Button variant="ghost" size="icon" class="h-7 w-7" onclick={handleRefresh}>
				<RefreshCwIcon class="h-3.5 w-3.5" />
			</Button>
		</div>

		<div class="border-b p-2">
			<div class="relative">
				<SearchIcon class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
				<Input
					type="text"
					placeholder="Search functions..."
					class="h-8 pl-8 text-sm"
					bind:value={searchQuery}
				/>
			</div>
		</div>

		<div class="flex-1 overflow-auto">
			{#if functionsQuery.isPending}
				<div class="space-y-2 p-2">
					{#each Array(5) as _}
						<Skeleton class="h-10 w-full" />
					{/each}
				</div>
			{:else if functionsQuery.isError}
				<div class="p-4 text-center">
					<p class="text-xs text-destructive">Failed to load functions</p>
				</div>
			{:else if filteredFunctions.length === 0}
				<div class="flex flex-col items-center justify-center p-6 text-center">
					<CodeIcon class="h-8 w-8 text-muted-foreground/50" />
					<p class="mt-2 text-xs text-muted-foreground">
						{searchQuery ? 'No functions match your search' : 'No functions found'}
					</p>
				</div>
			{:else}
				<div class="p-1">
					{#each filteredFunctions as func}
						<button
							class="w-full rounded-md px-2.5 py-2 text-left transition-colors hover:bg-accent {currentFunctionName ===
							func.name
								? 'bg-accent'
								: ''}"
							onclick={() => handleFunctionClick(func.name)}
						>
							<div class="flex items-center justify-between gap-2">
								<span class="truncate text-sm font-medium">{func.name}</span>
								{#if !func.enabled}
									<span class="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-muted-foreground/30"
									></span>
								{/if}
							</div>
							<div class="mt-1 flex items-center gap-2">
								<Badge variant="outline" class="text-xs {getRuntimeColor(func.runtime)}">
									{func.runtime}
								</Badge>
							</div>
						</button>
					{/each}
				</div>
			{/if}
		</div>
	</aside>

	<main class="flex-1 overflow-auto">
		{@render children()}
	</main>
</div>

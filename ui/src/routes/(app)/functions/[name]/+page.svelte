<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type FunctionDetail, type FunctionInvokeResponse } from '$lib/api/client';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import * as Tabs from '$ui/tabs';
	import { Badge } from '$ui/badge';
	import { Skeleton } from '$ui/skeleton';
	import { Button } from '$ui/button';
	import AlertCircleIcon from 'lucide-svelte/icons/alert-circle';
	import ArrowLeftIcon from 'lucide-svelte/icons/arrow-left';
	import { FunctionTestPanel, FunctionResponsePanel, FunctionHistory, type HistoryItem } from '$lib/components/functions';
	import { toast } from 'svelte-sonner';

	const name = $derived((page.params as { name?: string }).name);

	const functionQuery = createQuery(() => ({
		queryKey: ['admin', 'functions', name],
		queryFn: async (): Promise<FunctionDetail> => {
			if (!name) throw new Error('Function name is required');
			const result = await admin.functions.get(name);
			if (result.error) throw new Error(result.error.message);
			if (!result.data) throw new Error('Function not found');
			return result.data;
		},
		enabled: !!name
	}));

	let history = $state<HistoryItem[]>([]);
	let currentResponse = $state<FunctionInvokeResponse | null>(null);
	let isExecuting = $state(false);
	let lastFunctionName = $state('');

	$effect(() => {
		if (name && name !== lastFunctionName) {
			history = [];
			currentResponse = null;
			lastFunctionName = name;
		}
	});

	async function handleExecute(input: unknown, files?: File[]) {
		if (!name) return;

		isExecuting = true;
		const startTime = performance.now();
		try {
			const result = files && files.length > 0
				? await admin.functions.invokeWithFiles(name, input as Record<string, unknown>, files)
				: await admin.functions.invoke(name, input);
			const duration = Math.round(performance.now() - startTime);

			if (result.data) {
				const historyItem: HistoryItem = {
					id: crypto.randomUUID(),
					input,
					response: result.data,
					timestamp: new Date()
				};
				history = [historyItem, ...history].slice(0, 50);
				currentResponse = result.data;

				if (result.data.success) {
					toast.success(`Executed in ${result.data.duration_ms ?? duration}ms`);
				} else {
					toast.error('Execution failed', {
						description: result.data.error || 'Unknown error'
					});
				}
			} else if (result.error) {
				toast.error('Execution failed', {
					description: result.error.message || 'Unknown error'
				});
			}
		} catch (e) {
			const errorMessage = e instanceof Error ? e.message : 'Unknown error';
			toast.error('Execution failed', { description: errorMessage });
		} finally {
			isExecuting = false;
		}
	}

	function handleHistorySelect(item: HistoryItem) {
		currentResponse = item.response;
	}

	function handleHistoryClear() {
		history = [];
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

	function handleBack() {
		goto(`${base}/functions`);
	}
</script>

{#if functionQuery.isError && functionQuery.error?.message?.includes('not found')}
	<div class="flex h-full flex-col items-center justify-center p-8 text-center">
		<AlertCircleIcon class="h-12 w-12 text-muted-foreground/30" />
		<h2 class="mt-4 text-lg font-medium">Function not found</h2>
		<p class="mt-2 text-sm text-muted-foreground">
			The function "{name}" does not exist or has been removed.
		</p>
		<Button variant="outline" class="mt-6" onclick={handleBack}>
			<ArrowLeftIcon class="mr-2 h-4 w-4" />
			Back to Functions
		</Button>
	</div>
{:else}
	<div class="h-full flex flex-col">
		<header class="px-8 pt-8 pb-6">
			{#if functionQuery.isPending}
				<Skeleton class="h-8 w-48" />
				<Skeleton class="h-4 w-64 mt-2" />
			{:else if functionQuery.isError}
				<div class="flex items-center gap-2 text-destructive">
					<AlertCircleIcon class="h-5 w-5" />
					<span>Failed to load function</span>
				</div>
			{:else if functionQuery.data}
				{@const func = functionQuery.data}
				<div class="flex items-center gap-3">
					<h1 class="text-2xl font-semibold tracking-tight">{func.name}</h1>
					<Badge variant="outline" class={getRuntimeColor(func.runtime)}>
						{func.runtime}
					</Badge>
					{#if !func.enabled}
						<Badge variant="secondary">Disabled</Badge>
					{/if}
				</div>
				{#if func.description}
					<p class="mt-2 text-muted-foreground">{func.description}</p>
				{/if}
			{/if}
		</header>

		<Tabs.Root value="test" class="flex-1 flex flex-col min-h-0">
			<div class="px-8">
				<Tabs.List class="h-auto w-auto inline-flex bg-transparent p-0 gap-3">
					<Tabs.Trigger
						value="test"
						class="px-4 py-2.5 rounded-lg text-sm transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border border-border/20 hover:bg-muted/20 hover:border-border/30 data-[state=active]:bg-muted/30 data-[state=active]:border-border/40 data-[state=active]:text-foreground data-[state=active]:shadow-none"
					>
						Test
					</Tabs.Trigger>
					<Tabs.Trigger
						value="config"
						class="px-4 py-2.5 rounded-lg text-sm transition-colors bg-muted/10 backdrop-blur-lg backdrop-saturate-150 border border-border/20 hover:bg-muted/20 hover:border-border/30 data-[state=active]:bg-muted/30 data-[state=active]:border-border/40 data-[state=active]:text-foreground data-[state=active]:shadow-none"
					>
						Configuration
					</Tabs.Trigger>
				</Tabs.List>
			</div>

			<Tabs.Content value="test" class="flex-1 min-h-0 mt-0 p-8 pt-6">
				{#if functionQuery.isPending}
					<div class="space-y-4">
						<Skeleton class="h-64 w-full rounded-lg" />
					</div>
				{:else if functionQuery.data}
					{@const func = functionQuery.data}
					<div class="flex h-full flex-col gap-6">
						<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 flex-1 min-h-0">
							<div class="rounded-xl border border-border/50 overflow-hidden bg-card">
								<FunctionTestPanel
									functionName={func.name}
									sampleInput={func.sample_input}
									onExecute={handleExecute}
									isExecuting={isExecuting}
								/>
							</div>
							<div class="rounded-xl border border-border/50 overflow-hidden bg-card">
								<FunctionResponsePanel
									response={currentResponse}
									isLoading={isExecuting}
									autofocus={true}
								/>
							</div>
						</div>

						<div class="rounded-xl border border-border/50 overflow-hidden bg-card h-48 shrink-0">
							<FunctionHistory
								history={history}
								onSelect={handleHistorySelect}
								onClear={handleHistoryClear}
							/>
						</div>
					</div>
				{/if}
			</Tabs.Content>

			<Tabs.Content value="config" class="flex-1 min-h-0 mt-0 p-8 pt-6 overflow-auto">
				{#if functionQuery.isPending}
					<div class="max-w-2xl space-y-6">
						<Skeleton class="h-48 w-full rounded-xl" />
						<Skeleton class="h-32 w-full rounded-xl" />
					</div>
				{:else if functionQuery.data}
					{@const func = functionQuery.data}
					<div class="max-w-2xl space-y-8">
						<section class="rounded-xl border border-border/50 p-6 bg-card">
							<h2 class="text-lg font-medium mb-4">General</h2>
							<div class="space-y-4">
								<div class="grid gap-4 sm:grid-cols-2">
									<div>
										<p class="text-sm text-muted-foreground mb-1">Name</p>
										<p class="font-mono text-sm">{func.name}</p>
									</div>
									<div>
										<p class="text-sm text-muted-foreground mb-1">Runtime</p>
										<Badge variant="outline" class={getRuntimeColor(func.runtime)}>
											{func.runtime}
										</Badge>
									</div>
									<div>
										<p class="text-sm text-muted-foreground mb-1">Status</p>
										<Badge variant={func.enabled ? 'default' : 'secondary'}>
											{func.enabled ? 'Enabled' : 'Disabled'}
										</Badge>
									</div>
									{#if func.timeout}
										<div>
											<p class="text-sm text-muted-foreground mb-1">Timeout</p>
											<p class="text-sm">{func.timeout}</p>
										</div>
									{/if}
									{#if func.memory}
										<div>
											<p class="text-sm text-muted-foreground mb-1">Memory</p>
											<p class="text-sm">{func.memory}</p>
										</div>
									{/if}
								</div>

								<div class="pt-4 border-t border-border/50">
									<p class="text-sm text-muted-foreground mb-1">Path</p>
									<p class="font-mono text-sm bg-muted/50 px-3 py-2 rounded-lg">{func.path}</p>
								</div>
								<div>
									<p class="text-sm text-muted-foreground mb-1">Entrypoint</p>
									<p class="font-mono text-sm bg-muted/50 px-3 py-2 rounded-lg">{func.entrypoint}</p>
								</div>

								{#if func.description}
									<div class="pt-4 border-t border-border/50">
										<p class="text-sm text-muted-foreground mb-1">Description</p>
										<p class="text-sm">{func.description}</p>
									</div>
								{/if}
							</div>
						</section>

						{#if func.env && Object.keys(func.env).length > 0}
							<section class="rounded-xl border border-border/50 p-6 bg-card">
								<h2 class="text-lg font-medium mb-4">Environment Variables</h2>
								<div class="space-y-2">
									{#each Object.entries(func.env) as [key, value]}
										<div class="flex items-center gap-2 bg-muted/50 px-3 py-2 rounded-lg font-mono text-sm">
											<span>{key}</span>
											<span class="text-muted-foreground">=</span>
											<span class="text-muted-foreground">{value}</span>
										</div>
									{/each}
								</div>
							</section>
						{/if}

						{#if func.dependencies && func.dependencies.length > 0}
							<section class="rounded-xl border border-border/50 p-6 bg-card">
								<h2 class="text-lg font-medium mb-4">Dependencies</h2>
								<div class="flex flex-wrap gap-2">
									{#each func.dependencies as dep}
										<Badge variant="secondary">{dep}</Badge>
									{/each}
								</div>
							</section>
						{/if}
					</div>
				{/if}
			</Tabs.Content>
		</Tabs.Root>
	</div>
{/if}

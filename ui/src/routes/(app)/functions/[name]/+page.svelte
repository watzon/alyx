<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type FunctionDetail } from '$lib/api/client';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import * as Tabs from '$ui/tabs';
	import { Badge } from '$ui/badge';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import { Button } from '$ui/button';
	import ArrowLeftIcon from 'lucide-svelte/icons/arrow-left';
	import AlertCircleIcon from 'lucide-svelte/icons/alert-circle';
	import PlayIcon from 'lucide-svelte/icons/play';
	import SettingsIcon from 'lucide-svelte/icons/settings';

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

<div class="flex h-full flex-col">
	<!-- Header -->
	<header class="border-b bg-background px-6 py-4">
		<div class="flex items-start gap-4">
			<Button variant="ghost" size="icon" class="mt-1 h-8 w-8" onclick={handleBack}>
				<ArrowLeftIcon class="h-4 w-4" />
			</Button>

			<div class="flex-1 min-w-0">
				{#if functionQuery.isPending}
					<div class="space-y-2">
						<Skeleton class="h-8 w-48" />
						<Skeleton class="h-4 w-96" />
					</div>
				{:else if functionQuery.isError}
					<div class="flex items-center gap-2 text-destructive">
						<AlertCircleIcon class="h-5 w-5" />
						<span class="font-medium">
							{#if functionQuery.error?.message?.includes('not found')}
								Function "{name}" not found
							{:else}
								Failed to load function
							{/if}
						</span>
					</div>
				{:else if functionQuery.data}
					{@const func = functionQuery.data}
					<div class="flex items-center gap-3 flex-wrap">
						<h1 class="font-mono text-2xl font-semibold tracking-tight">{func.name}</h1>
						<Badge variant="outline" class={getRuntimeColor(func.runtime)}>
							{func.runtime}
						</Badge>
						{#if !func.enabled}
							<Badge variant="secondary">Disabled</Badge>
						{/if}
					</div>
					{#if func.description}
						<p class="mt-1 text-sm text-muted-foreground">{func.description}</p>
					{/if}
				{/if}
			</div>
		</div>
	</header>

	<!-- Tabs -->
	<div class="flex-1 overflow-auto">
		{#if functionQuery.isError && functionQuery.error?.message?.includes('not found')}
			<div class="flex h-full flex-col items-center justify-center p-8 text-center">
				<AlertCircleIcon class="h-12 w-12 text-muted-foreground/50" />
				<h2 class="mt-4 text-lg font-semibold">Function not found</h2>
				<p class="mt-1 text-sm text-muted-foreground">
					The function "{name}" does not exist or has been removed.
				</p>
				<Button class="mt-4" onclick={handleBack}>
					<ArrowLeftIcon class="mr-2 h-4 w-4" />
					Back to Functions
				</Button>
			</div>
		{:else}
			<Tabs.Root value="test" class="flex h-full flex-col">
				<div class="border-b px-6">
					<Tabs.List class="h-10 bg-transparent p-0">
						<Tabs.Trigger value="test" class="gap-2 rounded-none border-b-2 border-transparent px-4 py-2 data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none">
							<PlayIcon class="h-4 w-4" />
							Test
						</Tabs.Trigger>
						<Tabs.Trigger value="config" class="gap-2 rounded-none border-b-2 border-transparent px-4 py-2 data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none">
							<SettingsIcon class="h-4 w-4" />
							Config
						</Tabs.Trigger>
					</Tabs.List>
				</div>

				<!-- Test Tab -->
				<Tabs.Content value="test" class="flex-1 p-6">
					{#if functionQuery.isPending}
						<div class="space-y-4">
							<Skeleton class="h-8 w-48" />
							<Skeleton class="h-64 w-full" />
						</div>
					{:else if functionQuery.data}
						<div class="flex h-full flex-col items-center justify-center text-center">
							<PlayIcon class="h-12 w-12 text-muted-foreground/50" />
							<h3 class="mt-4 text-lg font-medium">Test Function</h3>
							<p class="mt-1 text-sm text-muted-foreground max-w-md">
								Test functionality will be implemented in the next task.
								This tab will allow you to invoke the function with custom input.
							</p>
						</div>
					{/if}
				</Tabs.Content>

				<!-- Config Tab -->
				<Tabs.Content value="config" class="flex-1 p-6">
					{#if functionQuery.isPending}
						<div class="space-y-4">
							<Skeleton class="h-8 w-48" />
							<Skeleton class="h-64 w-full" />
						</div>
					{:else if functionQuery.data}
						{@const func = functionQuery.data}
						<div class="mx-auto max-w-2xl space-y-6">
							<Card>
								<CardHeader>
									<CardTitle>Function Configuration</CardTitle>
									<CardDescription>Read-only view of function settings</CardDescription>
								</CardHeader>
								<CardContent class="space-y-4">
									<!-- Basic Info -->
									<div class="grid gap-4 sm:grid-cols-2">
										<div class="space-y-1">
											<p class="text-xs font-medium text-muted-foreground uppercase">Name</p>
											<p class="font-mono text-sm">{func.name}</p>
										</div>
										<div class="space-y-1">
											<p class="text-xs font-medium text-muted-foreground uppercase">Runtime</p>
											<Badge variant="outline" class={getRuntimeColor(func.runtime)}>
												{func.runtime}
											</Badge>
										</div>
										<div class="space-y-1">
											<p class="text-xs font-medium text-muted-foreground uppercase">Status</p>
											<Badge variant={func.enabled ? 'default' : 'secondary'}>
												{func.enabled ? 'Enabled' : 'Disabled'}
											</Badge>
										</div>
										{#if func.timeout}
											<div class="space-y-1">
												<p class="text-xs font-medium text-muted-foreground uppercase">Timeout</p>
												<p class="text-sm">{func.timeout}</p>
											</div>
										{/if}
										{#if func.memory}
											<div class="space-y-1">
												<p class="text-xs font-medium text-muted-foreground uppercase">Memory</p>
												<p class="text-sm">{func.memory}</p>
											</div>
										{/if}
									</div>

									<!-- Path & Entrypoint -->
									<div class="space-y-1">
										<p class="text-xs font-medium text-muted-foreground uppercase">Path</p>
										<p class="font-mono text-sm">{func.path}</p>
									</div>
									<div class="space-y-1">
										<p class="text-xs font-medium text-muted-foreground uppercase">Entrypoint</p>
										<p class="font-mono text-sm">{func.entrypoint}</p>
									</div>

									<!-- Description -->
									{#if func.description}
										<div class="space-y-1">
											<p class="text-xs font-medium text-muted-foreground uppercase">Description</p>
											<p class="text-sm">{func.description}</p>
										</div>
									{/if}
								</CardContent>
							</Card>

							<!-- Environment Variables -->
							{#if func.env && Object.keys(func.env).length > 0}
								<Card>
									<CardHeader>
										<CardTitle>Environment Variables</CardTitle>
									</CardHeader>
									<CardContent>
										<div class="space-y-2">
											{#each Object.entries(func.env) as [key, value]}
												<div class="flex items-center gap-2 rounded-md bg-muted px-3 py-2">
													<span class="font-mono text-sm">{key}</span>
													<span class="text-muted-foreground">=</span>
													<span class="font-mono text-sm text-muted-foreground">{value}</span>
												</div>
											{/each}
										</div>
									</CardContent>
								</Card>
							{/if}

							<!-- Dependencies -->
							{#if func.dependencies && func.dependencies.length > 0}
								<Card>
									<CardHeader>
										<CardTitle>Dependencies</CardTitle>
									</CardHeader>
									<CardContent>
										<div class="flex flex-wrap gap-2">
											{#each func.dependencies as dep}
												<Badge variant="secondary">{dep}</Badge>
											{/each}
										</div>
									</CardContent>
								</Card>
							{/if}
						</div>
					{/if}
				</Tabs.Content>
			</Tabs.Root>
		{/if}
	</div>
</div>

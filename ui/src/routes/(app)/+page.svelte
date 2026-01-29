<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { admin, type Stats, type Schema } from '$lib/api/client';
	import type { StorageStats } from '$lib/api/client';
	import * as Card from '$ui/card';
	import { Skeleton } from '$ui/skeleton';
	import DatabaseIcon from 'lucide-svelte/icons/database';
	import UsersIcon from 'lucide-svelte/icons/users';
	import CodeIcon from 'lucide-svelte/icons/code';
	import FileTextIcon from 'lucide-svelte/icons/file-text';
	import { metricsStore } from '$lib/stores/metrics.svelte';
	import * as Chart from '$ui/chart';
	import { AreaChart, BarChart } from 'layerchart';
	import { curveLinear } from 'd3-shape';

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

	const storageStatsQuery = createQuery(() => ({
		queryKey: ['admin', 'storage', 'stats'],
		queryFn: async (): Promise<StorageStats> => {
			const result = await admin.storageStats();
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

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
	}

	// Chart configurations
	const requestTrafficConfig = {
		total: { label: 'Total Requests', color: 'hsl(var(--primary))' },
		200: { label: '2xx Success', color: 'hsl(142 76% 36%)' },
		400: { label: '4xx Client Errors', color: 'hsl(38 92% 50%)' },
		500: { label: '5xx Server Errors', color: 'hsl(0 84% 60%)' }
	};

	const systemHealthConfig = {
		memory: { label: 'Memory Usage', color: 'hsl(var(--primary))' },
		goroutines: { label: 'Goroutines', color: 'hsl(280 65% 60%)' }
	};

	const storageConfig = {
		usage: { label: 'Storage Usage', color: 'hsl(var(--primary))' }
	};

	// Derived chart data
	const requestTrafficData = $derived(
		metricsStore.dataPoints.map((point) => ({
			time: new Date(point.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
			total: point.httpRequests,
			200: point.httpRequestsByStatus['200'] || 0,
			400: (point.httpRequestsByStatus['400'] || 0) + (point.httpRequestsByStatus['401'] || 0) + (point.httpRequestsByStatus['403'] || 0) + (point.httpRequestsByStatus['404'] || 0),
			500: (point.httpRequestsByStatus['500'] || 0) + (point.httpRequestsByStatus['502'] || 0) + (point.httpRequestsByStatus['503'] || 0)
		}))
	);

	const systemHealthData = $derived(
		metricsStore.dataPoints.map((point) => ({
			time: new Date(point.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
			memory: point.memoryBytes ? point.memoryBytes / (1024 * 1024) : 0, // Convert to MB
			goroutines: point.goroutines || 0
		}))
	);

	const storageData = $derived(
		storageStatsQuery.data?.buckets.map((bucket) => ({
			name: bucket.bucket,
			usage: bucket.totalBytes / (1024 * 1024) // Convert to MB
		})) || []
	);
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

	<div class="space-y-3">
		<Card.Root>
			<Card.Header class="pb-2">
				<Card.Title class="text-sm font-medium">Request Traffic</Card.Title>
			</Card.Header>
			<Card.Content>
				{#if requestTrafficData.length === 0}
					<div class="aspect-video flex items-center justify-center">
						<Skeleton class="h-full w-full" />
					</div>
				{:else}
					<Chart.Container config={requestTrafficConfig} class="aspect-video">
						<AreaChart
							data={requestTrafficData}
							x="time"
							series={[
								{ key: 'total', label: 'Total', color: requestTrafficConfig.total.color },
								{ key: '200', label: '2xx', color: requestTrafficConfig['200'].color },
								{ key: '400', label: '4xx', color: requestTrafficConfig['400'].color },
								{ key: '500', label: '5xx', color: requestTrafficConfig['500'].color }
							]}
							axis="x"
							props={{
								area: { curve: curveLinear, line: { class: 'stroke-1' }, 'fill-opacity': 0.3 },
								xAxis: { format: () => '' }
							}}
						>
							{#snippet tooltip()}
								<Chart.Tooltip indicator="line" />
							{/snippet}
						</AreaChart>
					</Chart.Container>
				{/if}
			</Card.Content>
		</Card.Root>

		<div class="grid gap-3 md:grid-cols-2">
			<Card.Root>
				<Card.Header class="pb-2">
					<Card.Title class="text-sm font-medium">System Health</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if systemHealthData.length === 0}
						<div class="aspect-video flex items-center justify-center">
							<Skeleton class="h-full w-full" />
						</div>
					{:else}
						<Chart.Container config={systemHealthConfig} class="aspect-video">
							<AreaChart
								data={systemHealthData}
								x="time"
								series={[
									{ key: 'memory', label: 'Memory (MB)', color: systemHealthConfig.memory.color },
									{ key: 'goroutines', label: 'Goroutines', color: systemHealthConfig.goroutines.color }
								]}
								axis="x"
								props={{
									area: { curve: curveLinear, line: { class: 'stroke-1' }, 'fill-opacity': 0.3 },
									xAxis: { format: () => '' }
								}}
							>
								{#snippet tooltip()}
									<Chart.Tooltip indicator="line" />
								{/snippet}
							</AreaChart>
						</Chart.Container>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="pb-2">
					<Card.Title class="text-sm font-medium">Storage Usage</Card.Title>
				</Card.Header>
				<Card.Content>
					{#if storageStatsQuery.isPending}
						<div class="aspect-video flex items-center justify-center">
							<Skeleton class="h-full w-full" />
						</div>
					{:else if storageData.length === 0}
						<div class="aspect-video flex flex-col items-center justify-center text-center space-y-2">
							<DatabaseIcon class="h-8 w-8 text-muted-foreground/50" />
							<p class="text-sm text-muted-foreground">No storage buckets configured</p>
						</div>
					{:else}
						<Chart.Container config={storageConfig} class="aspect-video">
							<BarChart
								data={storageData}
								x="name"
								series={[{ key: 'usage', label: 'Usage (MB)', color: storageConfig.usage.color }]}
								axis="x"
								props={{
									bars: { stroke: 'none', rounded: 'all', radius: 8 },
									xAxis: { format: () => '' }
								}}
							>
								{#snippet tooltip()}
									<Chart.Tooltip indicator="line" />
								{/snippet}
							</BarChart>
						</Chart.Container>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	</div>
</div>

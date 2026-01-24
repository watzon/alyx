<script lang="ts">
	import '../app.css';
	import { Toaster } from '$ui/sonner';
	import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
	import { onMount } from 'svelte';
	import { authStore } from '$lib/stores/auth.svelte';

	let { children } = $props();

	const queryClient = new QueryClient({
		defaultOptions: {
			queries: {
				staleTime: 1000 * 60,
				refetchOnWindowFocus: false
			}
		}
	});

	onMount(() => {
		authStore.initialize();
	});
</script>

<QueryClientProvider client={queryClient}>
	{@render children()}
	<Toaster />
</QueryClientProvider>

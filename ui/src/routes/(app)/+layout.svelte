<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { configStore } from '$lib/stores/config.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { base, resolve } from '$app/paths';
	import LogOutIcon from 'lucide-svelte/icons/log-out';
	import BookOpenIcon from 'lucide-svelte/icons/book-open';
	import ExternalLinkIcon from 'lucide-svelte/icons/external-link';
	import * as DropdownMenu from '$ui/dropdown-menu';
	import * as Avatar from '$ui/avatar';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Overview' },
		{ href: '/collections', label: 'Collections' },
		{ href: '/storage', label: 'Storage' },
		{ href: '/schema', label: 'Schema' },
		{ href: '/functions', label: 'Functions' },
		{ href: '/users', label: 'Users' },
		{ href: '/logs', label: 'Logs' },
		{ href: '/settings', label: 'Settings' }
	];

	function isActive(href: string): boolean {
		const pathname: string = page.url.pathname;
		if (href === '/') {
			return pathname === base || pathname === `${base}/`;
		}
		return pathname.startsWith(`${base}${href}`);
	}

	async function handleLogout() {
		await authStore.logout();
		goto(resolve('/(auth)/login'));
	}

	$effect(() => {
		if (!authStore.isLoading && !authStore.isAuthenticated) {
			goto(resolve('/(auth)/login'));
		}
	});

	$effect(() => {
		if (authStore.isAuthenticated && !configStore.isLoading && !configStore.config) {
			configStore.load();
		}
	});
</script>

{#if authStore.isLoading}
	<div class="flex h-screen items-center justify-center bg-background">
		<div class="text-sm text-muted-foreground">Loading...</div>
	</div>
{:else if authStore.isAuthenticated}
	<div class="min-h-screen bg-background">
		<header class="sticky top-0 z-50 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
			<div class="px-4 sm:px-6 lg:px-8">
				<div class="flex h-14 items-center justify-between">
					<div class="flex items-center gap-4">
						<a href="{base}/" class="flex items-center">
							<img src="{base}/alyx-logo.svg" alt="Alyx" class="h-6" />
						</a>
					</div>

					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<button
									class="flex items-center gap-2 rounded-full p-0.5 hover:bg-accent transition-colors"
									{...props}
								>
									<Avatar.Root class="h-7 w-7">
										<Avatar.Fallback class="bg-muted text-xs">
											{authStore.user?.email?.charAt(0).toUpperCase() ?? 'U'}
										</Avatar.Fallback>
									</Avatar.Root>
								</button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end" class="w-56">
							<DropdownMenu.Label class="font-normal">
								<div class="flex flex-col space-y-0.5">
									<p class="text-sm font-medium">{authStore.user?.email}</p>
									<p class="text-xs text-muted-foreground">{authStore.user?.role}</p>
								</div>
							</DropdownMenu.Label>
							<DropdownMenu.Separator />
							{#if configStore.docsEnabled}
								<DropdownMenu.Item onclick={() => window.open(configStore.docsUrl, '_blank')}>
									<BookOpenIcon class="mr-2 h-4 w-4" />
									Documentation
									<ExternalLinkIcon class="ml-auto h-3 w-3 opacity-50" />
								</DropdownMenu.Item>
							{/if}
							<DropdownMenu.Item onclick={handleLogout}>
								<LogOutIcon class="mr-2 h-4 w-4" />
								Log out
							</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>

				<nav class="flex items-center gap-1 -mb-px">
					{#each navItems as item}
						<a
							href="{base}{item.href === '/' ? '' : item.href}"
							class="relative px-3 py-3 text-sm transition-colors {isActive(item.href)
								? 'text-foreground'
								: 'text-muted-foreground hover:text-foreground'}"
						>
							{item.label}
							{#if isActive(item.href)}
								<span class="absolute inset-x-0 -bottom-px h-px bg-foreground"></span>
							{/if}
						</a>
					{/each}
				</nav>
			</div>
		</header>

		<main class="px-4 sm:px-6 lg:px-8 py-6">
			{@render children()}
		</main>
	</div>
{:else}
	<div class="flex h-screen items-center justify-center bg-background">
		<div class="text-sm text-muted-foreground">Redirecting to login...</div>
	</div>
{/if}
